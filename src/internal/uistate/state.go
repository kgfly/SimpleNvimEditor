// Package uistate implements the client-side state machine for the Neovim
// UI protocol (see `:h ui.txt` / `:h ui-events` in Neovim). It has no
// dependency on Gio or on the RPC transport: it only knows how to turn
// batches of decoded `redraw` events into an in-memory model of grids,
// highlights, cursor, and window placement that a renderer can draw.
package uistate

import "sync"

// State is the full client-side mirror of what Nvim's UI protocol has told
// us. All mutation happens through Apply; reads should go through the
// accessor methods, which take the internal lock.
type State struct {
	mu sync.RWMutex

	grids   map[int]*Grid
	windows *WindowSet
	hl      *HighlightTable
	cursor  Cursor

	mode      string
	modeIdx   int
	modeInfos []ModeInfo
	busy      bool
}

// New returns an empty State, ready to receive redraw batches.
func New() *State {
	return &State{
		grids:   make(map[int]*Grid),
		windows: newWindowSet(),
		hl:      newHighlightTable(),
	}
}

// Apply decodes and applies one `redraw` notification, which is a batch of
// `[event-name, args...]` tuples as delivered by go-client's "redraw"
// handler (see nvimproc.Process.Redraw). It returns true if the batch
// contained a `flush` event, meaning the renderer should draw a new frame.
func (s *State) Apply(batch [][]interface{}) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	flushed := false
	for _, tuple := range batch {
		if len(tuple) == 0 {
			continue
		}
		name, _ := tuple[0].(string)
		args := tuple[1:]

		switch name {
		case "flush":
			flushed = true
		case "grid_resize":
			s.applyGridResize(args)
		case "grid_line":
			s.applyGridLine(args)
		case "grid_clear":
			s.applyGridClear(args)
		case "grid_destroy":
			s.applyGridDestroy(args)
		case "grid_cursor_goto":
			s.applyGridCursorGoto(args)
		case "grid_scroll":
			s.applyGridScroll(args)
		case "win_pos":
			s.applyWinPos(args)
		case "win_float_pos":
			s.applyWinFloatPos(args)
		case "win_hide":
			s.applyWinHide(args)
		case "win_close":
			s.applyWinClose(args)
		case "msg_set_pos":
			s.applyMsgSetPos(args)
		case "default_colors_set":
			s.hl.applyDefaultColorsSet(args)
		case "hl_attr_define":
			s.hl.applyHlAttrDefine(args)
		case "mode_info_set":
			s.applyModeInfoSet(args)
		case "mode_change":
			s.applyModeChange(args)
		case "busy_start":
			s.busy = true
		case "busy_stop":
			s.busy = false
		default:
			// Unhandled events (popupmenu_*, cmdline_*, tabline_update,
			// msg_showcmd, win_viewport, ...) are intentionally ignored in
			// this phase; see IMPLEMENTATION_PLAN.md for the roadmap.
			// msg_set_pos IS handled above: without ext_cmdline/ext_messages,
			// Nvim still routes the command line and messages through their
			// own grid (not grid 1) whenever ext_multigrid is on, and
			// msg_set_pos is the only event that says where that grid goes.
		}
	}
	return flushed
}

// gridLocked returns the grid with the given id, creating it if necessary.
// Callers must hold s.mu.
func (s *State) gridLocked(id int) *Grid {
	g, ok := s.grids[id]
	if !ok {
		g = newGrid(id)
		s.grids[id] = g
	}
	return g
}

// Snapshot is a read-only, self-contained copy of the state that is cheap
// enough to build once per frame and safe to read without holding any lock.
type Snapshot struct {
	Grids     map[int]GridView
	Windows   []Placement
	Highlight HighlightView
	Cursor    Cursor
	Mode      string
	ModeIdx   int
	ModeInfos []ModeInfo
	Busy      bool
}

// Snapshot copies the current state out for rendering. It is intentionally
// simple (full copies, no sharing) because grid sizes in a terminal-like UI
// are small enough that this is cheap relative to a GPU frame.
func (s *State) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	grids := make(map[int]GridView, len(s.grids))
	for id, g := range s.grids {
		grids[id] = g.view()
	}

	return Snapshot{
		Grids:     grids,
		Windows:   s.windows.ordered(),
		Highlight: s.hl.view(),
		Cursor:    s.cursor,
		Mode:      s.mode,
		ModeIdx:   s.modeIdx,
		ModeInfos: append([]ModeInfo(nil), s.modeInfos...),
		Busy:      s.busy,
	}
}
