package uistate

import "sort"

// Placement describes where one grid sits within the app window, per the
// `win_pos` / `win_float_pos` events of the multigrid UI protocol.
type Placement struct {
	GridID int
	Row    int // top-left row, in cells, relative to the base grid
	Col    int // top-left column, in cells, relative to the base grid
	Width  int
	Height int
	Float  bool
	Hidden bool
	// ZIndex orders overlapping grids for drawing: higher paints later
	// (on top). The base grid (id 1) always has ZIndex 0.
	ZIndex int
}

// WindowSet tracks the placement of every non-destroyed grid.
type WindowSet struct {
	byGrid map[int]*Placement
	nextZ  int
}

func newWindowSet() *WindowSet {
	return &WindowSet{byGrid: make(map[int]*Placement)}
}

func (w *WindowSet) remove(gridID int) {
	delete(w.byGrid, gridID)
}

func (w *WindowSet) upsert(p Placement) {
	existing, ok := w.byGrid[p.GridID]
	if !ok {
		w.nextZ++
		p.ZIndex = w.nextZ
		w.byGrid[p.GridID] = &p
		return
	}
	p.ZIndex = existing.ZIndex
	*existing = p
}

// ordered returns all visible placements sorted so that normal (split)
// windows are drawn before floating windows, and within each group by
// arrival order, so later floats/splits correctly paint over earlier ones.
func (w *WindowSet) ordered() []Placement {
	out := make([]Placement, 0, len(w.byGrid))
	for _, p := range w.byGrid {
		if !p.Hidden {
			out = append(out, *p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Float != out[j].Float {
			return !out[i].Float // non-floats first
		}
		return out[i].ZIndex < out[j].ZIndex
	})
	return out
}

// HitTest finds the topmost visible grid at the given base-grid cell
// position and returns its grid ID and grid-relative (row, col). When
// ext_multigrid is active, editor content lives on grids 2+ placed via
// win_pos / win_float_pos; sending mouse events to grid 1 is wrong
// because Nvim interprets the coordinates relative to grid 1, which is
// just the chrome area, not the text. The caller should use the returned
// grid and coordinates instead of blindly passing grid=1 to
// nvim_input_mouse.
//
// Windows are checked in reverse z-order (topmost first) so that floats
// and overlays win over underlying splits. If no window contains the
// position, ok is false — the caller can fall back to grid=1.
func HitTest(windows []Placement, row, col int) (grid, gridRow, gridCol int, ok bool) {
	// Walk the list backwards: ordered() puts later/higher z-index
	// entries at the end, so the topmost visible window is last.
	for i := len(windows) - 1; i >= 0; i-- {
		p := windows[i]
		if row >= p.Row && row < p.Row+p.Height &&
			col >= p.Col && col < p.Col+p.Width {
			return p.GridID, row - p.Row, col - p.Col, true
		}
	}
	return 0, 0, 0, false
}

// applyWinPos handles a normal (non-floating) window placement update:
// [grid, win, start_row, start_col, width, height].
func (s *State) applyWinPos(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 6 {
			continue
		}
		s.windows.upsert(Placement{
			GridID: toInt(t[0]),
			Row:    toInt(t[2]),
			Col:    toInt(t[3]),
			Width:  toInt(t[4]),
			Height: toInt(t[5]),
		})
	}
}

// applyWinFloatPos handles a floating window placement update:
// [grid, win, anchor, anchor_grid, anchor_row, anchor_col, focusable, zindex?].
//
// Known limitation: anchor_grid is assumed to be the base grid (1). Nested
// float-on-float anchoring is not yet resolved to absolute coordinates; see
// IMPLEMENTATION_PLAN.md Phase 2.
func (s *State) applyWinFloatPos(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 6 {
			continue
		}
		gridID := toInt(t[0])
		row := toInt(t[4])
		col := toInt(t[5])

		width, height := 0, 0
		if g, ok := s.grids[gridID]; ok {
			width, height = g.Cols, g.Rows
		}

		s.windows.upsert(Placement{
			GridID: gridID,
			Row:    row,
			Col:    col,
			Width:  width,
			Height: height,
			Float:  true,
		})
	}
}

// applyWinHide handles `win_hide`: [grid]. The grid keeps its content but
// stops being drawn until a future win_pos/win_float_pos shows it again.
func (s *State) applyWinHide(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 1 {
			continue
		}
		if p, ok := s.windows.byGrid[toInt(t[0])]; ok {
			p.Hidden = true
		}
	}
}

// applyWinClose handles `win_close`: [grid]. Unlike win_hide, the window is
// gone for good (though grid_destroy is what actually frees the grid).
func (s *State) applyWinClose(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 1 {
			continue
		}
		s.windows.remove(toInt(t[0]))
	}
}

// applyMsgSetPos handles `msg_set_pos`: [grid, row, scrolled, sep_char].
// Whenever ext_multigrid is on but ext_cmdline/ext_messages are off (our
// case), Nvim doesn't draw the command line and messages into grid 1 —
// it puts them on their own grid and uses this event to say where that
// grid sits: at the given row, spanning the full width of the base grid,
// overlaying whatever was there. row == -1 means "not currently shown"
// (e.g. right after the message area closes). Without handling this event,
// that grid's content is applied to state but never placed, so it's built
// but never drawn — which is exactly why ":" appeared to do nothing.
func (s *State) applyMsgSetPos(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 2 {
			continue
		}
		gridID := toInt(t[0])
		row := toInt(t[1])

		width, height := 0, 0
		if g, ok := s.grids[gridID]; ok {
			width, height = g.Cols, g.Rows
		}

		s.windows.upsert(Placement{
			GridID: gridID,
			Row:    row,
			Col:    0,
			Width:  width,
			Height: height,
			Float:  true, // draw on top of splits, like any overlay
			Hidden: row < 0,
		})
	}
}
