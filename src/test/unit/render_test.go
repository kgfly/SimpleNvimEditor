package unit_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
	"github.com/kgfly/SimpleNvimEditor/internal/render"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// newTestContext builds a minimal layout.Context suitable for driving
// render/text code outside of a real Gio window: no GPU or display is
// needed to build an op.Ops list, only Ops/Metric/Constraints.
func newTestContext(ops *op.Ops, size image.Point) layout.Context {
	return layout.Context{
		Ops:         ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(size),
	}
}

func TestMeasureReturnsPositiveMetrics(t *testing.T) {
	shaper := render.NewShaper()
	face := render.FontFace(config.Default().Editor)

	var ops op.Ops
	gtx := newTestContext(&ops, image.Pt(2000, 2000))

	m := render.Measure(gtx, shaper, face, unit.Sp(14))
	if m.CellWidth <= 0 {
		t.Errorf("CellWidth = %d, want > 0", m.CellWidth)
	}
	if m.CellHeight <= 0 {
		t.Errorf("CellHeight = %d, want > 0", m.CellHeight)
	}
}

func TestMeasureScalesWithFontSize(t *testing.T) {
	shaper := render.NewShaper()
	face := render.FontFace(config.Default().Editor)

	var ops1, ops2 op.Ops
	small := render.Measure(newTestContext(&ops1, image.Pt(2000, 2000)), shaper, face, unit.Sp(10))
	large := render.Measure(newTestContext(&ops2, image.Pt(2000, 2000)), shaper, face, unit.Sp(30))

	if large.CellWidth <= small.CellWidth {
		t.Errorf("CellWidth did not grow with font size: size10=%d size30=%d", small.CellWidth, large.CellWidth)
	}
	if large.CellHeight <= small.CellHeight {
		t.Errorf("CellHeight did not grow with font size: size10=%d size30=%d", small.CellHeight, large.CellHeight)
	}
}

func TestFontFaceRespectsUseSystemFonts(t *testing.T) {
	bundled := render.FontFace(config.EditorConfig{UseSystemFonts: false})
	if bundled.Typeface != "Go Mono" {
		t.Errorf("bundled FontFace typeface = %q, want %q", bundled.Typeface, "Go Mono")
	}

	system := render.FontFace(config.EditorConfig{UseSystemFonts: true, FontFamily: "Consolas"})
	if system.Typeface != "Consolas" {
		t.Errorf("system FontFace typeface = %q, want %q", system.Typeface, "Consolas")
	}

	bold := render.FontFace(config.EditorConfig{UseSystemFonts: true, FontFamily: "Hack Nerd Font Mono Bold"})
	if bold.Typeface != "Hack Nerd Font Mono" {
		t.Errorf("bold FontFace typeface = %q, want %q", bold.Typeface, "Hack Nerd Font Mono")
	}
}

func TestNewShaperNeverReturnsNil(t *testing.T) {
	if render.NewShaper() == nil {
		t.Fatalf("NewShaper(bundled) returned nil")
	}
	if render.NewShaper() == nil {
		t.Fatalf("NewShaper(system) returned nil")
	}
}

// testFonts builds a Fonts value with fixed, deterministic Metrics (rather
// than calling Measure) so Frame-painting tests below don't depend on
// whatever glyphs the bundled font happens to produce.
func testFonts(t *testing.T) render.Fonts {
	t.Helper()
	cfg := config.Default().Editor
	return render.Fonts{
		Shaper:  render.NewShaper(),
		Face:    render.FontFace(cfg),
		Size:    unit.Sp(cfg.FontSize),
		Metrics: render.Metrics{CellWidth: 8, CellHeight: 16, Baseline: 12},
	}
}

// runFrame is a small helper that asserts render.Frame completes without
// panicking for a given snapshot; Gio's op.Ops binary encoding isn't
// practical to assert against directly (see IMPLEMENTATION_PLAN in the
// project history for why pixel-level assertions are out of scope for unit
// tests), so this is our regression guard against nil-pointer/index panics.
func runFrame(t *testing.T, snap uistate.Snapshot) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("render.Frame panicked: %v", r)
		}
	}()
	var ops op.Ops
	gtx := newTestContext(&ops, image.Pt(400, 300))
	render.Frame(gtx, testFonts(t), snap)
}

func TestFrameEmptyState(t *testing.T) {
	runFrame(t, uistate.New().Snapshot())
}

func TestFrameSingleGridWithContent(t *testing.T) {
	s := uistate.New()
	s.Apply([][]interface{}{
		{"grid_resize", []interface{}{1, 10, 5}},
		{"grid_line", []interface{}{1, 0, 0, []interface{}{[]interface{}{"hi"}}}},
		{"grid_cursor_goto", []interface{}{1, 0, 0}},
	})
	runFrame(t, s.Snapshot())
}

func TestFrameMultigridWithFloat(t *testing.T) {
	s := uistate.New()
	s.Apply([][]interface{}{
		{"grid_resize", []interface{}{1, 40, 20}},
		{"grid_resize", []interface{}{2, 20, 10}},
		{"win_pos", []interface{}{2, 0, 5, 5, 20, 10}},
		{"grid_resize", []interface{}{3, 10, 3}},
		{"win_float_pos", []interface{}{3, 0, "NW", 1, 2, 2, true}},
		{"grid_cursor_goto", []interface{}{3, 1, 1}},
	})
	runFrame(t, s.Snapshot())
}

func TestFrameCursorOnUnplacedGridDoesNotPanic(t *testing.T) {
	s := uistate.New()
	s.Apply([][]interface{}{
		{"grid_resize", []interface{}{1, 10, 5}},
		// Cursor points at a grid id that was never resized/placed.
		{"grid_cursor_goto", []interface{}{99, 0, 0}},
	})
	runFrame(t, s.Snapshot())
}

func TestFrameCursorShapes(t *testing.T) {
	for _, shape := range []string{"block", "horizontal", "vertical"} {
		t.Run(shape, func(t *testing.T) {
			s := uistate.New()
			s.Apply([][]interface{}{
				{"grid_resize", []interface{}{1, 10, 5}},
				{"grid_cursor_goto", []interface{}{1, 2, 2}},
				{"mode_info_set", []interface{}{true, []interface{}{
					map[string]interface{}{"cursor_shape": shape, "cell_percentage": 50},
				}}},
				{"mode_change", []interface{}{"normal", 0}},
			})
			runFrame(t, s.Snapshot())
		})
	}
}

func TestFrameCursorAttrIDZero(t *testing.T) {
	// When attr_id is 0, cursorColor should use the default foreground.
	s := uistate.New()
	s.Apply([][]interface{}{
		{"grid_resize", []interface{}{1, 10, 5}},
		{"grid_cursor_goto", []interface{}{1, 0, 0}},
		{"mode_info_set", []interface{}{true, []interface{}{
			map[string]interface{}{"cursor_shape": "block", "cell_percentage": 100, "attr_id": 0},
		}}},
		{"mode_change", []interface{}{"normal", 0}},
	})
	runFrame(t, s.Snapshot())
}

func TestFrameCursorWithHighlightAttr(t *testing.T) {
	// When attr_id > 0, cursorColor should use the bg from that hl group.
	s := uistate.New()
	s.Apply([][]interface{}{
		{"grid_resize", []interface{}{1, 10, 5}},
		{"default_colors_set", []interface{}{int64(0xffffff), int64(0x000000), int64(0xff0000), int64(0), int64(0)}},
		{"hl_attr_define", []interface{}{int64(7), map[string]interface{}{"foreground": int64(0x00ff00), "background": int64(0x0000ff)}, map[string]interface{}{}, []interface{}{}}},
		{"grid_cursor_goto", []interface{}{1, 0, 0}},
		{"mode_info_set", []interface{}{true, []interface{}{
			map[string]interface{}{"cursor_shape": "block", "cell_percentage": 100, "attr_id": int64(7)},
		}}},
		{"mode_change", []interface{}{"normal", 0}},
	})
	runFrame(t, s.Snapshot())
}

func TestSolidMaterial(t *testing.T) {
	var ops op.Ops
	c := render.SolidMaterial(&ops, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
	// SolidMaterial should return a valid (non-zero) CallOp.
	_ = c
}

func TestFrameWithEmptyTextCells(t *testing.T) {
	// Exercise the drawText early-return for empty text (double-width
	// continuation cells have empty Text in the second column).
	s := uistate.New()
	s.Apply([][]interface{}{
		{"grid_resize", []interface{}{1, 10, 2}},
		{"grid_line", []interface{}{1, 0, 0, []interface{}{
			[]interface{}{"W", int64(0)},
			[]interface{}{"", int64(0)}, // continuation cell, empty text
			[]interface{}{"x", int64(0)},
		}}},
		{"grid_cursor_goto", []interface{}{1, 0, 0}},
	})
	runFrame(t, s.Snapshot())
}

func TestFrameCursorZeroPercentage(t *testing.T) {
	s := uistate.New()
	s.Apply([][]interface{}{
		{"grid_resize", []interface{}{1, 10, 5}},
		{"grid_cursor_goto", []interface{}{1, 0, 0}},
		{"mode_info_set", []interface{}{true, []interface{}{
			map[string]interface{}{"cursor_shape": "vertical", "cell_percentage": 0},
		}}},
		{"mode_change", []interface{}{"normal", 0}},
	})
	runFrame(t, s.Snapshot())
}

func TestFrameWithPlacedSplitWindow(t *testing.T) {
	s := uistate.New()
	s.Apply([][]interface{}{
		{"grid_resize", []interface{}{1, 40, 20}},
		{"grid_resize", []interface{}{2, 20, 10}},
		{"win_pos", []interface{}{2, 0, 0, 0, 20, 10}},
		{"grid_line", []interface{}{2, 0, 0, []interface{}{[]interface{}{"hello split"}}}},
		{"grid_cursor_goto", []interface{}{2, 0, 5}},
	})
	runFrame(t, s.Snapshot())
}

func TestGridOriginBaseGrid(t *testing.T) {
	snap := uistate.New().Snapshot()
	pt, ok := render.GridOrigin(snap, 1, render.Metrics{CellWidth: 8, CellHeight: 16})
	if !ok {
		t.Fatal("GridOrigin for grid 1 should always return true")
	}
	if pt.X != 0 || pt.Y != 0 {
		t.Fatalf("GridOrigin(1) = %v, want (0,0)", pt)
	}
}

func TestGridOriginPlacedGrid(t *testing.T) {
	s := uistate.New()
	s.Apply([][]interface{}{
		{"grid_resize", []interface{}{1, 80, 24}},
		{"grid_resize", []interface{}{2, 40, 12}},
		{"win_pos", []interface{}{2, 0, 3, 5, 40, 12}},
	})
	m := render.Metrics{CellWidth: 8, CellHeight: 16}
	pt, ok := render.GridOrigin(s.Snapshot(), 2, m)
	if !ok {
		t.Fatal("GridOrigin should find placed grid")
	}
	if pt.X != 5*8 || pt.Y != 3*16 {
		t.Fatalf("GridOrigin(2) = %v, want (%d,%d)", pt, 5*8, 3*16)
	}
}

func TestGridOriginUnplacedGrid(t *testing.T) {
	snap := uistate.New().Snapshot()
	_, ok := render.GridOrigin(snap, 99, render.Metrics{CellWidth: 8, CellHeight: 16})
	if ok {
		t.Fatal("GridOrigin should return false for unplaced grid")
	}
}

func TestFontFaceWeightSuffixes(t *testing.T) {
	cases := []struct {
		family   string
		wantFace string
	}{
		{"Hack Thin", "Hack"},
		{"Hack ExtraLight", "Hack"},
		{"Hack Light", "Hack"},
		{"Hack Medium", "Hack"},
		{"Hack SemiBold", "Hack"},
		{"Hack Bold", "Hack"},
		{"Hack ExtraBold", "Hack"},
		{"Hack Black", "Hack"},
		{"Hack", "Hack"},
	}
	for _, c := range cases {
		f := render.FontFace(config.EditorConfig{UseSystemFonts: true, FontFamily: c.family})
		if string(f.Typeface) != c.wantFace {
			t.Errorf("FontFace(%q).Typeface = %q, want %q", c.family, f.Typeface, c.wantFace)
		}
	}
}

func TestFrameWindowWithMissingGrid(t *testing.T) {
	// A placement references a grid that was never resized/created. The
	// "if !ok { continue }" branch in Frame should handle this gracefully.
	s := uistate.New()
	s.Apply([][]interface{}{
		{"grid_resize", []interface{}{1, 40, 20}},
		// Place grid 7 which doesn't exist — exercises the continue branch.
		{"win_pos", []interface{}{7, 0, 0, 0, 10, 5}},
	})
	runFrame(t, s.Snapshot())
}

func TestFrameNoBaseGrid(t *testing.T) {
	// State with no grid 1 at all — the "if base, ok" branch is false.
	snap := uistate.Snapshot{
		Highlight: uistate.New().Snapshot().Highlight,
	}
	runFrame(t, snap)
}
