package unit_test

import (
	"testing"

	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

func TestWinPosCreatesPlacement(t *testing.T) {
	s := uistate.New()
	// win_pos args: [grid, win, start_row, start_col, width, height]
	s.Apply(batch(ev("win_pos", args(2, 0, 3, 4, 20, 10))))

	snap := s.Snapshot()
	if len(snap.Windows) != 1 {
		t.Fatalf("got %d placements, want 1", len(snap.Windows))
	}
	p := snap.Windows[0]
	want := uistate.Placement{GridID: 2, Row: 3, Col: 4, Width: 20, Height: 10, ZIndex: 1}
	if p != want {
		t.Fatalf("placement = %+v, want %+v", p, want)
	}
}

func TestWinFloatPosDerivesSizeFromGrid(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(9, 15, 5)),
		// win_float_pos args: [grid, win, anchor, anchor_grid, anchor_row, anchor_col, focusable]
		ev("win_float_pos", args(9, 0, "NW", 1, 2, 6, true)),
	))

	snap := s.Snapshot()
	if len(snap.Windows) != 1 {
		t.Fatalf("got %d placements, want 1", len(snap.Windows))
	}
	p := snap.Windows[0]
	if !p.Float {
		t.Fatalf("placement.Float = false, want true")
	}
	if p.Row != 2 || p.Col != 6 {
		t.Fatalf("placement pos = (%d,%d), want (2,6)", p.Row, p.Col)
	}
	if p.Width != 15 || p.Height != 5 {
		t.Fatalf("placement size = (%d,%d), want (15,5) derived from grid_resize", p.Width, p.Height)
	}
}

func TestOrderedPutsNormalWindowsBeforeFloats(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(3, 5, 5)),
		ev("win_float_pos", args(3, 0, "NW", 1, 0, 0, true)), // float created first
		ev("win_pos", args(2, 0, 0, 0, 5, 5)),                // normal split created second
	))

	windows := s.Snapshot().Windows
	if len(windows) != 2 {
		t.Fatalf("got %d placements, want 2", len(windows))
	}
	if windows[0].Float {
		t.Fatalf("first placement is a float; normal windows should sort first: %+v", windows)
	}
	if !windows[1].Float {
		t.Fatalf("second placement is not a float: %+v", windows)
	}
}

func TestWinHideExcludesFromOrdered(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("win_pos", args(2, 0, 0, 0, 5, 5))))
	if len(s.Snapshot().Windows) != 1 {
		t.Fatalf("setup failed: expected one placement")
	}

	s.Apply(batch(ev("win_hide", args(2))))
	if got := len(s.Snapshot().Windows); got != 0 {
		t.Fatalf("got %d visible placements after win_hide, want 0", got)
	}
}

func TestWinPosAfterHideMakesVisibleAgain(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("win_pos", args(2, 0, 0, 0, 5, 5)),
		ev("win_hide", args(2)),
		ev("win_pos", args(2, 0, 1, 1, 5, 5)),
	))
	windows := s.Snapshot().Windows
	if len(windows) != 1 {
		t.Fatalf("got %d placements, want 1 visible again", len(windows))
	}
	if windows[0].Hidden {
		t.Fatalf("placement still marked hidden after a fresh win_pos")
	}
}

func TestWinCloseRemovesPlacement(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("win_pos", args(2, 0, 0, 0, 5, 5))))
	s.Apply(batch(ev("win_close", args(2))))

	if got := len(s.Snapshot().Windows); got != 0 {
		t.Fatalf("got %d placements after win_close, want 0", got)
	}
}

func TestMsgSetPosPlacesMessageGridOverTheBaseGrid(t *testing.T) {
	s := uistate.New()
	// This is exactly the sequence a real Nvim sends the moment you press
	// ":" with ext_multigrid on but ext_cmdline/ext_messages off: the
	// cmdline text lands on its own grid (3), and msg_set_pos is the only
	// event that says where it belongs. Without handling it, that grid's
	// content is never placed and ":" silently appears to do nothing.
	s.Apply(batch(
		ev("grid_resize", args(3, 125, 1)),
		ev("msg_set_pos", args(3, 37, false, " ")),
	))

	snap := s.Snapshot()
	if len(snap.Windows) != 1 {
		t.Fatalf("got %d placements, want 1", len(snap.Windows))
	}
	p := snap.Windows[0]
	if p.GridID != 3 || p.Row != 37 || p.Col != 0 {
		t.Fatalf("placement = %+v, want grid 3 at (37,0)", p)
	}
	if p.Width != 125 || p.Height != 1 {
		t.Fatalf("placement size = (%d,%d), want (125,1) derived from grid_resize", p.Width, p.Height)
	}
	if p.Hidden {
		t.Fatalf("placement.Hidden = true, want false for row >= 0")
	}
	if !p.Float {
		t.Fatalf("placement.Float = false, want true so it draws over split windows")
	}
}

func TestMsgSetPosRowNegativeOneHidesTheGrid(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(3, 125, 1)),
		ev("msg_set_pos", args(3, 37, false, " ")),
		ev("msg_set_pos", args(3, -1, false, " ")),
	))

	if got := len(s.Snapshot().Windows); got != 0 {
		t.Fatalf("got %d visible placements after row=-1, want 0", got)
	}
}

func TestHitTestFindsGridContainingPosition(t *testing.T) {
	windows := []uistate.Placement{
		{GridID: 2, Row: 0, Col: 0, Width: 40, Height: 24},
	}
	grid, gridRow, gridCol, ok := uistate.HitTest(windows, 5, 10)
	if !ok {
		t.Fatalf("HitTest missed a position inside the only window")
	}
	if grid != 2 || gridRow != 5 || gridCol != 10 {
		t.Fatalf("HitTest = (grid=%d, row=%d, col=%d), want (2, 5, 10)", grid, gridRow, gridCol)
	}
}

func TestHitTestTranslatesToGridRelativeCoordinates(t *testing.T) {
	// Window starts at (3, 4); a click at (7, 12) should land at (4, 8)
	// relative to the grid's own origin.
	windows := []uistate.Placement{
		{GridID: 5, Row: 3, Col: 4, Width: 20, Height: 20},
	}
	grid, gridRow, gridCol, ok := uistate.HitTest(windows, 7, 12)
	if !ok || grid != 5 || gridRow != 4 || gridCol != 8 {
		t.Fatalf("HitTest = (grid=%d, row=%d, col=%d, ok=%v), want (5, 4, 8, true)", grid, gridRow, gridCol, ok)
	}
}

func TestHitTestReturnsFalseWhenNothingContainsPosition(t *testing.T) {
	windows := []uistate.Placement{
		{GridID: 2, Row: 0, Col: 0, Width: 10, Height: 10},
	}
	// Just past the bottom-right corner: row/col == Row+Height / Col+Width
	// is exclusive, so this must miss.
	_, _, _, ok := uistate.HitTest(windows, 10, 10)
	if ok {
		t.Fatalf("HitTest matched a position exactly on the window's exclusive edge")
	}

	_, _, _, ok = uistate.HitTest(nil, 0, 0)
	if ok {
		t.Fatalf("HitTest matched against an empty window list")
	}
}

func TestHitTestPrefersTopmostOverlappingWindow(t *testing.T) {
	// Both windows cover (5,5). ordered() places the topmost window last,
	// so HitTest -- which walks backwards -- must return the second one.
	windows := []uistate.Placement{
		{GridID: 2, Row: 0, Col: 0, Width: 20, Height: 20},
		{GridID: 3, Row: 0, Col: 0, Width: 20, Height: 20},
	}
	grid, _, _, ok := uistate.HitTest(windows, 5, 5)
	if !ok || grid != 3 {
		t.Fatalf("HitTest = (grid=%d, ok=%v), want the topmost (last) grid 3", grid, ok)
	}
}

func TestUpsertPreservesZIndexAcrossUpdates(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("win_pos", args(2, 0, 0, 0, 5, 5)), // grid 2 gets ZIndex 1
		ev("win_pos", args(3, 0, 0, 0, 5, 5)), // grid 3 gets ZIndex 2
	))
	// Re-position grid 2; it should keep its original arrival order (ZIndex),
	// not jump to the front/back.
	s.Apply(batch(ev("win_pos", args(2, 0, 9, 9, 5, 5))))

	windows := s.Snapshot().Windows
	var z2, z3 int
	for _, p := range windows {
		switch p.GridID {
		case 2:
			z2 = p.ZIndex
		case 3:
			z3 = p.ZIndex
		}
	}
	if z2 != 1 || z3 != 2 {
		t.Fatalf("z-indices = grid2:%d grid3:%d, want grid2:1 grid3:2 (stable across updates)", z2, z3)
	}
}
