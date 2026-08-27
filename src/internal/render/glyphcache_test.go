package render

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// newContext returns a layout.Context rendering at the given pixel density.
func newContext(ops *op.Ops, px image.Point, scale float32) layout.Context {
	return layout.Context{
		Ops:         ops,
		Metric:      unit.Metric{PxPerDp: scale, PxPerSp: scale},
		Constraints: layout.Exact(px),
	}
}

// testFonts builds a real shaper and measures cells at the given scale.
func testFonts(t testing.TB, scale float32, px image.Point) Fonts {
	t.Helper()
	cfg := config.Default()
	f := Fonts{
		Shaper: NewShaper(cfg.Editor),
		Face:   FontFace(cfg.Editor),
		Size:   unit.Sp(cfg.Editor.FontSize),
	}
	var ops op.Ops
	f.Metrics = Measure(newContext(&ops, px, scale), f.Shaper, f.Face, f.Size)
	return f
}

// gridSnapshot builds a snapshot whose cells cycle through a few characters
// and highlights, approximating real screen content.
func gridSnapshot(cols, rows int) uistate.Snapshot {
	const alphabet = "abcdefgh 1234"
	data := make([][]uistate.Cell, rows)
	for r := range data {
		data[r] = make([]uistate.Cell, cols)
		for c := range data[r] {
			data[r][c] = uistate.Cell{
				Text: string(alphabet[(r+c)%len(alphabet)]),
				HlID: (r + c) % 3,
			}
		}
	}
	return uistate.Snapshot{
		Grids:  map[int]uistate.GridView{1: {ID: 1, Rows: rows, Cols: cols, Data: data}},
		Cursor: uistate.Cursor{GridID: 1},
	}
}

// TestGlyphCacheReusesRepeatedCells is the unit-level regression test for
// the editor stalling while a window is resized.
//
// Terminal grids are highly repetitive, so shaping every cell independently
// repeats the same expensive work tens of thousands of times per frame. The
// cache must collapse that to one entry per distinct glyph, and the entry
// count must not grow with the number of cells drawn.
func TestGlyphCacheReusesRepeatedCells(t *testing.T) {
	px := image.Pt(800, 600)
	fonts := testFonts(t, 1, px)

	var ops op.Ops
	gtx := newContext(&ops, px, 1)
	cache := newGlyphCache(gtx, fonts)

	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for i := 0; i < 5000; i++ {
		cache.get(gtx, "a", white)
	}
	if n := len(cache.calls); n != 1 {
		t.Fatalf("5000 identical cells produced %d cache entries, want 1", n)
	}

	// A different color is a genuinely different drawing, so it must not
	// reuse the previous recording.
	cache.get(gtx, "a", color.NRGBA{R: 255, A: 255})
	if n := len(cache.calls); n != 2 {
		t.Errorf("same glyph in a new color gave %d entries, want 2", n)
	}

	// ...and so is a different character.
	cache.get(gtx, "b", white)
	if n := len(cache.calls); n != 3 {
		t.Errorf("new glyph gave %d entries, want 3", n)
	}
}

// rasterize renders through Gio's headless GPU backend and returns the
// resulting pixels, so tests can compare what the user actually sees rather
// than a proxy for it. It skips when no GPU context is available.
func rasterize(t *testing.T, px image.Point, draw func(gtx layout.Context)) *image.RGBA {
	t.Helper()
	w, err := headless.NewWindow(px.X, px.Y)
	if err != nil {
		t.Skipf("headless GPU unavailable: %v", err)
	}
	defer w.Release()

	var ops op.Ops
	draw(newContext(&ops, px, 1))

	if err := w.Frame(&ops); err != nil {
		t.Fatalf("headless frame: %v", err)
	}
	img := image.NewRGBA(image.Rectangle{Max: px})
	if err := w.Screenshot(img); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	return img
}

// drawUncached reproduces the pre-cache drawing path, shaping every cell on
// its own. It exists solely as the reference the cached renderer is
// compared against, so an optimization that quietly changed the output
// cannot pass unnoticed.
func drawUncached(gtx layout.Context, fonts Fonts, snap uistate.Snapshot) {
	defFg, defBg := snap.Highlight.DefaultColors()
	size := gtx.Constraints.Max
	paint.FillShape(gtx.Ops, defBg, clip.Rect(image.Rect(0, 0, size.X, size.Y)).Op())

	gv := snap.Grids[1]
	cw, ch := fonts.Metrics.CellWidth, fonts.Metrics.CellHeight
	for row, cells := range gv.Data {
		y := row * ch
		col := 0
		for col < len(cells) {
			runStart := col
			hlID := cells[col].HlID
			for col < len(cells) && cells[col].HlID == hlID {
				col++
			}
			x := runStart * cw
			fg, bg := snap.Highlight.Resolve(hlID)
			paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rect(x, y, x+(col-runStart)*cw, y+ch)).Op())
			for i, cell := range cells[runStart:col] {
				if cell.Text == "" || cell.Text == " " {
					continue
				}
				off := op.Offset(image.Pt(x+i*cw, y)).Push(gtx.Ops)
				m := op.Record(gtx.Ops)
				paint.ColorOp{Color: fg}.Add(gtx.Ops)
				labelWidget(gtx, fonts, cell.Text, m.Stop())
				off.Pop()
			}
		}
	}
	drawCursor(gtx, fonts, snap, defFg)
}

// TestCachedRenderMatchesUncached is the correctness proof for the glyph
// cache: replaying a recorded glyph must produce pixel-identical output to
// shaping every cell independently. A faster renderer that draws even
// slightly different pixels is a regression, not an optimization.
func TestCachedRenderMatchesUncached(t *testing.T) {
	px := image.Pt(400, 300)
	fonts := testFonts(t, 1, px)
	snap := gridSnapshot(40, 15)

	cached := rasterize(t, px, func(gtx layout.Context) {
		Frame(gtx, fonts, snap)
	})
	uncached := rasterize(t, px, func(gtx layout.Context) {
		drawUncached(gtx, fonts, snap)
	})

	if !bytes.Equal(cached.Pix, uncached.Pix) {
		t.Error("cached rendering differs from per-cell shaping; the cache changed what the user sees")
	}
}

// TestFrameIsDeterministic guards against the cache introducing frame-to-
// frame variation: the same snapshot must always rasterize identically.
func TestFrameIsDeterministic(t *testing.T) {
	px := image.Pt(400, 300)
	fonts := testFonts(t, 1, px)
	snap := gridSnapshot(40, 15)

	draw := func(gtx layout.Context) { Frame(gtx, fonts, snap) }
	first := rasterize(t, px, draw)
	if got := rasterize(t, px, draw); !bytes.Equal(got.Pix, first.Pix) {
		t.Error("the same snapshot rasterized differently on a second frame")
	}
}

// TestFrameRendersAtEitherDisplayDensity is the cross-monitor regression
// test. A low-DPI panel has smaller cells and therefore *more* of them for
// the same physical screen, which is exactly the case where the per-cell
// cost hurt most: the 3440x1378 external monitor yields a 491x91 grid
// (~45k cells) versus ~23k on the Retina built-in.
func TestFrameRendersAtEitherDisplayDensity(t *testing.T) {
	for _, tc := range []struct {
		name  string
		scale float32
		px    image.Point
	}{
		{"retina-2x", 2, image.Pt(2400, 1536)},
		{"external-1x", 1, image.Pt(3440, 1378)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fonts := testFonts(t, tc.scale, tc.px)
			if fonts.Metrics.CellWidth < 1 || fonts.Metrics.CellHeight < 1 {
				t.Fatalf("bad metrics at scale %v: %+v", tc.scale, fonts.Metrics)
			}
			cols := tc.px.X / fonts.Metrics.CellWidth
			rows := tc.px.Y / fonts.Metrics.CellHeight

			var ops op.Ops
			gtx := newContext(&ops, tc.px, tc.scale)
			Frame(gtx, fonts, gridSnapshot(cols, rows))
		})
	}
}
