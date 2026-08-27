package unit_test

import (
	"image/color"
	"testing"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

func nrgba(r, g, b uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}
}

// Exercise the len(t)<N guard paths and uncommon branches.

func TestApplyGridResizeTooFewArgs(t *testing.T) {
	s := uistate.New()
	// Only 2 elements instead of 3; should be silently ignored.
	s.Apply(batch(ev("grid_resize", args(1, 10))))
	if len(s.Snapshot().Grids) != 0 {
		t.Fatal("malformed grid_resize should not create a grid")
	}
}

func TestApplyGridLineTooFewArgs(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(1, 5, 1)),
		ev("grid_line", args(1, 0, 0)), // missing cells array
	))
	// Should not panic; row stays blank.
	for _, cell := range s.Snapshot().Grids[1].Data[0] {
		if cell.Text != " " {
			t.Fatalf("cell = %q, want blank", cell.Text)
		}
	}
}

func TestApplyGridClearTooFewArgs(t *testing.T) {
	s := uistate.New()
	// Empty tuple for grid_clear; should not panic.
	s.Apply(batch(ev("grid_clear")))
}

func TestApplyGridDestroyTooFewArgs(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("grid_destroy")))
}

func TestApplyGridScrollTooFewArgs(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(1, 5, 5)),
		ev("grid_scroll", args(1, 0, 5, 0)), // only 4 args, need 7
	))
}

func TestGridScrollWithRegion(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("grid_resize", args(1, 4, 4))))
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			marker := string(rune('a' + r*4 + c))
			s.Apply(batch(ev("grid_line", args(1, r, c, []interface{}{[]interface{}{marker}}))))
		}
	}

	// Scroll only columns [1,3) of rows [1,3) up by 1.
	s.Apply(batch(ev("grid_scroll", args(1, 1, 3, 1, 3, 1, 0))))

	g := s.Snapshot().Grids[1]
	// Row 1 cols 1-2 should have content from row 2 cols 1-2.
	if got := g.Data[1][1].Text; got != "j" {
		t.Fatalf("data[1][1] = %q, want %q", got, "j")
	}
	// Row 2 cols 1-2 should be blanked.
	if got := g.Data[2][1].Text; got != " " {
		t.Fatalf("data[2][1] = %q, want blank", got)
	}
	// Row 1 col 0 (outside scroll region) should be unchanged.
	if got := g.Data[1][0].Text; got != "e" {
		t.Fatalf("data[1][0] = %q, want %q (untouched by scroll)", got, "e")
	}
}

func TestGridScrollNegativeWithRegion(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("grid_resize", args(1, 3, 4))))
	for r := 0; r < 4; r++ {
		marker := string(rune('a' + r))
		s.Apply(batch(ev("grid_line", args(1, r, 0, []interface{}{[]interface{}{marker}}))))
	}

	// Scroll rows [1,3) down by 1 (negative rowsBy).
	s.Apply(batch(ev("grid_scroll", args(1, 1, 3, 0, 3, -1, 0))))

	g := s.Snapshot().Grids[1]
	// Row 1 should be blanked (newly exposed).
	if got := g.Data[1][0].Text; got != " " {
		t.Fatalf("data[1][0] = %q, want blank", got)
	}
	// Row 2 should have what was in row 1.
	if got := g.Data[2][0].Text; got != "b" {
		t.Fatalf("data[2][0] = %q, want %q", got, "b")
	}
	// Row 0 should be untouched (outside region).
	if got := g.Data[0][0].Text; got != "a" {
		t.Fatalf("data[0][0] = %q, want %q", got, "a")
	}
}

func TestGridLineEmptyTuple(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(1, 5, 1)),
		ev("grid_line", args(1, 0, 0, []interface{}{
			[]interface{}{}, // empty cell tuple
			[]interface{}{"a"},
		})),
	))
	// Empty tuple should be skipped; "a" goes at col 0 (since empty was skipped).
	if got := s.Snapshot().Grids[1].Data[0][0].Text; got != "a" {
		t.Fatalf("cell[0][0] = %q, want %q", got, "a")
	}
}

func TestGridLineColBeyondWidth(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(1, 2, 1)),
		ev("grid_line", args(1, 0, 0, []interface{}{
			[]interface{}{"x", 0, 5}, // repeat 5 into a 2-wide grid
		})),
	))
	g := s.Snapshot().Grids[1]
	if g.Data[0][0].Text != "x" || g.Data[0][1].Text != "x" {
		t.Fatalf("visible cells wrong: %+v", g.Data[0])
	}
}

func TestApplyWinPosTooFewArgs(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("win_pos", args(2, 0, 1)))) // need 6 args
	if len(s.Snapshot().Windows) != 0 {
		t.Fatal("malformed win_pos should not create a placement")
	}
}

func TestApplyWinFloatPosTooFewArgs(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("win_float_pos", args(2, 0)))) // need 6 args
	if len(s.Snapshot().Windows) != 0 {
		t.Fatal("malformed win_float_pos should not create a placement")
	}
}

func TestApplyWinFloatPosDerivesSize(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(5, 20, 10)),
		ev("win_float_pos", args(5, 0, "NW", 1, 2, 3, true)),
	))
	snap := s.Snapshot()
	found := false
	for _, w := range snap.Windows {
		if w.GridID == 5 {
			found = true
			if w.Width != 20 || w.Height != 10 {
				t.Fatalf("float size = %dx%d, want 20x10", w.Width, w.Height)
			}
			if !w.Float {
				t.Fatal("expected Float=true")
			}
		}
	}
	if !found {
		t.Fatal("float window placement not found")
	}
}

func TestApplyWinFloatPosNoGrid(t *testing.T) {
	s := uistate.New()
	// Float pos for grid that doesn't exist yet — width/height should be 0.
	s.Apply(batch(ev("win_float_pos", args(99, 0, "NW", 1, 0, 0, true))))
	for _, w := range s.Snapshot().Windows {
		if w.GridID == 99 {
			if w.Width != 0 || w.Height != 0 {
				t.Fatalf("expected 0x0 for unknown grid, got %dx%d", w.Width, w.Height)
			}
			return
		}
	}
	t.Fatal("placement for grid 99 not found")
}

func TestApplyWinHideTooFewArgs(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("win_hide"))) // no args
}

func TestApplyWinCloseTooFewArgs(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("win_close"))) // no args
}

func TestApplyWinCloseEmptyInvocation(t *testing.T) {
	s := uistate.New()
	// Invocation with empty tuple — len < 1 guard.
	s.Apply(batch([]interface{}{"win_close", []interface{}{}}))
}

func TestApplyWinCloseNonexistentGrid(t *testing.T) {
	s := uistate.New()
	// Close a grid that was never placed.
	s.Apply(batch(ev("win_close", args(999))))
}

func TestApplyMsgSetPosTooFewArgs(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("msg_set_pos", args(1)))) // need 2 args
}

func TestApplyGridCursorGotoTooFewArgs(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("grid_cursor_goto", args(1, 0)))) // need 3
}

func TestApplyModeInfoSetTooFewArgs(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("mode_info_set", args(true)))) // need 2
}

func TestApplyModeChangeTooFewArgs(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("mode_change", args("normal")))) // need 2
}

func TestApplySetTitleEmptyArgs(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("set_title"))) // no args
	if s.Snapshot().Title != "" {
		t.Fatal("empty set_title should not change title")
	}
}

func TestApplySetTitleNonSliceRow(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("set_title", args(42)))) // non-[]interface{} row
	if s.Snapshot().Title != "" {
		t.Fatal("non-slice set_title should not change title")
	}
}

func TestApplySetTitleEmptySliceRow(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("set_title", []interface{}{})))
	if s.Snapshot().Title != "" {
		t.Fatal("empty row should not change title")
	}
}

func TestApplyHlAttrDefineTooFewArgs(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("hl_attr_define", args(1)))) // need 2 (id + map)
}

func TestApplyDefaultColorsSetTooFewArgs(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("default_colors_set", args(0xFF0000, 0x000000)))) // need 3
}

func TestCurrentModeInfoOutOfBounds(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("mode_info_set", args(true, []interface{}{
			map[string]interface{}{"cursor_shape": "vertical", "cell_percentage": int64(25)},
		})),
		ev("mode_change", args("normal", int64(99))), // index out of bounds
	))
	info := s.Snapshot().CurrentModeInfo()
	// Out-of-bounds falls back to the default block cursor.
	if info.CursorShape != "block" || info.CellPercentage != 100 {
		t.Fatalf("out-of-bounds mode index should return default block, got %+v", info)
	}
}

func TestCurrentModeInfoDefaultFallback(t *testing.T) {
	s := uistate.New()
	// No mode_info_set or mode_change sent.
	info := s.Snapshot().CurrentModeInfo()
	// Defaults to a block cursor per the function's doc.
	if info.CursorShape != "block" || info.CellPercentage != 100 {
		t.Fatalf("default ModeInfo should be block/100, got %+v", info)
	}
}

func TestApplyBusyStartStopToggle(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("busy_start")))
	if !s.Snapshot().Busy {
		t.Fatal("expected Busy=true after busy_start")
	}
	s.Apply(batch(ev("busy_stop")))
	if s.Snapshot().Busy {
		t.Fatal("expected Busy=false after busy_stop")
	}
}

func TestApplySetTitleColonPreserved(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("set_title", args("Dir:sub myfile.go"))))
	got := s.Snapshot().Title
	if got != "Dir:sub myfile.go" {
		t.Fatalf("title = %q, want %q", got, "Dir:sub myfile.go")
	}
}

func TestApplySetTitleNoColon(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("set_title", args("myfile.go"))))
	got := s.Snapshot().Title
	if got != "myfile.go" {
		t.Fatalf("title = %q, want %q", got, "myfile.go")
	}
}

func TestApplySetTitleColonNoSpace(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("set_title", args("file:nospace"))))
	got := s.Snapshot().Title
	if got == "" {
		t.Fatal("title should not be empty")
	}
}

func TestGridScrollClampsNegativeRegion(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("grid_resize", args(1, 3, 3))))
	for r := 0; r < 3; r++ {
		s.Apply(batch(ev("grid_line", args(1, r, 0, []interface{}{[]interface{}{string(rune('a' + r))}}))))
	}
	// Negative top/left, oversized bot/right — should be clamped.
	s.Apply(batch(ev("grid_scroll", args(1, -5, 99, -5, 99, 1, 0))))
	g := s.Snapshot().Grids[1]
	// Effectively scrolls the full grid up by 1.
	if g.Data[0][0].Text != "b" {
		t.Fatalf("data[0][0] = %q, want %q", g.Data[0][0].Text, "b")
	}
	if g.Data[2][0].Text != " " {
		t.Fatalf("data[2][0] = %q, want blank", g.Data[2][0].Text)
	}
}

func TestGridScrollLargeRowsByClamps(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("grid_resize", args(1, 1, 3))))
	for r := 0; r < 3; r++ {
		s.Apply(batch(ev("grid_line", args(1, r, 0, []interface{}{[]interface{}{string(rune('x' + r))}}))))
	}
	// rowsBy larger than region — should blank everything.
	s.Apply(batch(ev("grid_scroll", args(1, 0, 3, 0, 1, 100, 0))))
	g := s.Snapshot().Grids[1]
	for r := 0; r < 3; r++ {
		if g.Data[r][0].Text != " " {
			t.Fatalf("data[%d][0] = %q, want blank", r, g.Data[r][0].Text)
		}
	}
}

func TestApplyWinHideNonexistentGrid(t *testing.T) {
	s := uistate.New()
	// Hide a grid that was never placed — should not panic.
	s.Apply(batch(ev("win_hide", args(999))))
}

func TestApplyWinHideEmptyInvocation(t *testing.T) {
	s := uistate.New()
	// Invocation with empty tuple — len < 1 guard.
	s.Apply(batch([]interface{}{"win_hide", []interface{}{}}))
}

func TestApplyGridDestroyEmptyInvocation(t *testing.T) {
	s := uistate.New()
	// Invocation with empty tuple — len < 1 guard.
	s.Apply(batch([]interface{}{"grid_destroy", []interface{}{}}))
}

func TestApplyGridClearEmptyInvocation(t *testing.T) {
	s := uistate.New()
	// grid_clear with empty invocation tuple.
	s.Apply(batch([]interface{}{"grid_clear", []interface{}{}}))
}

func TestApplyGridDestroyNonexistentGrid(t *testing.T) {
	s := uistate.New()
	// Destroy a grid that doesn't exist — should not panic.
	s.Apply(batch(ev("grid_destroy", args(999))))
}

func TestGridLineNegativeColStart(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(1, 5, 1)),
		// Negative col start: first cells should be skipped, then write at valid cols.
		ev("grid_line", args(1, 0, -2, []interface{}{
			[]interface{}{"a"},
			[]interface{}{"b"},
			[]interface{}{"c"},
			[]interface{}{"d"},
		})),
	))
	g := s.Snapshot().Grids[1]
	// col -2 and -1 are out of bounds (skipped), col 0 gets "c", col 1 gets "d".
	if g.Data[0][0].Text != "c" {
		t.Fatalf("data[0][0] = %q, want %q", g.Data[0][0].Text, "c")
	}
	if g.Data[0][1].Text != "d" {
		t.Fatalf("data[0][1] = %q, want %q", g.Data[0][1].Text, "d")
	}
}

func TestApplyMsgSetPosWithNegativeRow(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(5, 10, 2)),
		ev("msg_set_pos", args(5, -1, false, "~")),
	))
	snap := s.Snapshot()
	// With row=-1, the placement is hidden, so it shouldn't appear in Windows.
	for _, w := range snap.Windows {
		if w.GridID == 5 {
			t.Fatal("msg_set_pos with row=-1 should be hidden (not in Windows)")
		}
	}
}

func TestApplyMsgSetPosWithPositiveRow(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(5, 10, 2)),
		ev("msg_set_pos", args(5, 3, false, "~")),
	))
	snap := s.Snapshot()
	found := false
	for _, w := range snap.Windows {
		if w.GridID == 5 {
			found = true
			if w.Row != 3 {
				t.Fatalf("row = %d, want 3", w.Row)
			}
			if !w.Float {
				t.Fatal("msg_set_pos placements should be Float=true")
			}
		}
	}
	if !found {
		t.Fatal("msg_set_pos should produce a visible placement")
	}
}

func TestToIntAllTypes(t *testing.T) {
	s := uistate.New()
	// Use different integer encodings that exercise toInt branches.
	// int64 (standard msgpack)
	s.Apply(batch(ev("grid_resize", args(int64(1), int64(5), int64(3)))))
	g := s.Snapshot().Grids[1]
	if g.Cols != 5 || g.Rows != 3 {
		t.Fatalf("int64: cols=%d rows=%d, want 5x3", g.Cols, g.Rows)
	}
	// uint64
	s.Apply(batch(ev("grid_resize", args(uint64(2), uint64(4), uint64(2)))))
	g2 := s.Snapshot().Grids[2]
	if g2.Cols != 4 || g2.Rows != 2 {
		t.Fatalf("uint64: cols=%d rows=%d, want 4x2", g2.Cols, g2.Rows)
	}
	// int
	s.Apply(batch(ev("grid_resize", args(3, 6, 1))))
	g3 := s.Snapshot().Grids[3]
	if g3.Cols != 6 || g3.Rows != 1 {
		t.Fatalf("int: cols=%d rows=%d, want 6x1", g3.Cols, g3.Rows)
	}
	// Non-numeric (default branch → 0): use a string for grid_id.
	// This should create grid 0 with 0x0 size (all convert to 0).
	s.Apply(batch(ev("grid_resize", args("not_a_number", "foo", "bar"))))
	g0 := s.Snapshot().Grids[0]
	if g0.Cols != 0 || g0.Rows != 0 {
		t.Fatalf("default: cols=%d rows=%d, want 0x0", g0.Cols, g0.Rows)
	}
}

func TestDefaultFontFamilyForOS(t *testing.T) {
	// Just verify it returns a non-empty string on any platform.
	f := config.DefaultFontFamilyForOS()
	if f == "" {
		t.Fatal("DefaultFontFamilyForOS returned empty string")
	}
}

func TestHighlightResolveReverseSwapsColors(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("default_colors_set", args(int64(0xffffff), int64(0x000000), int64(0))),
		ev("hl_attr_define", args(int64(1), map[string]interface{}{
			"foreground": int64(0xff0000),
			"background": int64(0x00ff00),
			"reverse":    true,
		})),
	))
	snap := s.Snapshot()
	fg, bg := snap.Highlight.Resolve(1)
	// Reverse swaps fg and bg.
	if fg.R != 0x00 || fg.G != 0xff {
		t.Fatalf("reversed fg = %+v, want green", fg)
	}
	if bg.R != 0xff || bg.G != 0x00 {
		t.Fatalf("reversed bg = %+v, want red", bg)
	}
}

func TestHighlightDefaultColors(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("default_colors_set", args(int64(0xaabbcc), int64(0x112233), int64(0))),
	))
	snap := s.Snapshot()
	fg, bg := snap.Highlight.DefaultColors()
	if fg.R != 0xaa || fg.G != 0xbb || fg.B != 0xcc {
		t.Fatalf("default fg = %+v, want 0xaabbcc", fg)
	}
	if bg.R != 0x11 || bg.G != 0x22 || bg.B != 0x33 {
		t.Fatalf("default bg = %+v, want 0x112233", bg)
	}
}

func TestHighlightAttrBoolFlags(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("hl_attr_define", args(int64(2), map[string]interface{}{
			"bold":          true,
			"italic":        true,
			"underline":     true,
			"undercurl":     true,
			"strikethrough": true,
		})),
	))
	attr := s.Snapshot().Highlight.Attr(2)
	if !attr.Bold || !attr.Italic || !attr.Underline || !attr.Undercurl || !attr.Strike {
		t.Fatalf("attr flags = %+v, expected all true", attr)
	}
}

func TestHighlightResolveUnknownIDReturnsDefaults(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("default_colors_set", args(int64(0xffffff), int64(0x000000), int64(0))),
	))
	fg, bg := s.Snapshot().Highlight.Resolve(999)
	if fg.R != 0xff || fg.G != 0xff || fg.B != 0xff {
		t.Fatalf("unknown hlID fg = %+v, want white", fg)
	}
	if bg.R != 0x00 || bg.G != 0x00 || bg.B != 0x00 {
		t.Fatalf("unknown hlID bg = %+v, want black", bg)
	}
}

func TestHighlightSpecialColor(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("hl_attr_define", args(int64(3), map[string]interface{}{
			"special": int64(0x0000ff),
		})),
	))
	// Just verify the attr was stored without panicking; Special is
	// an internal field used by undercurl rendering.
	_ = s.Snapshot().Highlight.Attr(3)
}

func TestGridScrollPartialRegionDown(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("grid_resize", args(1, 4, 5))))
	for r := 0; r < 5; r++ {
		for c := 0; c < 4; c++ {
			m := string(rune('A' + r*4 + c))
			s.Apply(batch(ev("grid_line", args(1, r, c, []interface{}{[]interface{}{m}}))))
		}
	}
	// Scroll columns [2,4) of rows [1,4) down by 2.
	s.Apply(batch(ev("grid_scroll", args(1, 1, 4, 2, 4, -2, 0))))
	g := s.Snapshot().Grids[1]
	// Rows 1-2 cols 2-3 should be blanked (newly exposed).
	if g.Data[1][2].Text != " " {
		t.Fatalf("data[1][2] = %q, want blank", g.Data[1][2].Text)
	}
	if g.Data[2][2].Text != " " {
		t.Fatalf("data[2][2] = %q, want blank", g.Data[2][2].Text)
	}
	// Row 3 cols 2-3 should have old row 1 cols 2-3 content.
	if g.Data[3][2].Text != "G" {
		t.Fatalf("data[3][2] = %q, want %q", g.Data[3][2].Text, "G")
	}
}
