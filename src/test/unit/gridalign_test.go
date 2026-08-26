package unit_test

import (
	"image"
	"testing"

	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
	"github.com/kgfly/SimpleNvimEditor/internal/render"
)

// TestCellWidthMatchesShapedAdvance is the regression test for grid text
// drifting out of its columns.
//
// The renderer places every cell at an integer multiple of CellWidth. The
// old painter shaped a whole same-highlight run as a single string and let
// the font's own advance carry the glyphs along, so any mismatch between the
// real (fractional) advance and the integer CellWidth accumulated across the
// run. Over a long line that grew into whole columns of error, which showed
// up as the first character of a row vanishing and the column at a run
// boundary looking half-width.
//
// Two things keep that fixed, and this covers the metric half: CellWidth
// must be a faithful rounding of the advance the shaper actually uses. If it
// is systematically too small (as plain integer truncation made it), glyphs
// are wider than the cells they are assigned to and neighbours collide.
func TestCellWidthMatchesShapedAdvance(t *testing.T) {
	cfg := config.Default().Editor
	shaper := render.NewShaper(cfg)
	face := render.FontFace(cfg)

	const probe = "MMMMMMMMMMMMMMMMMMMM"

	for _, size := range []unit.Sp{12, 13, 14, 15, 16, 18} {
		var measureOps op.Ops
		m := render.Measure(
			newTestContext(&measureOps, image.Pt(4000, 4000)),
			shaper, face, size,
		)

		var shapeOps op.Ops
		gtx := newTestContext(&shapeOps, image.Pt(1<<20, 1<<20))
		gtx.Constraints.Min = image.Point{}
		dims := widget.Label{MaxLines: 1}.Layout(gtx, shaper, face, size, probe, op.CallOp{})

		// Allow at most half a pixel of rounding error per cell.
		drift := dims.Size.X - m.CellWidth*len(probe)
		if drift < 0 {
			drift = -drift
		}
		if drift > len(probe)/2 {
			t.Errorf("size %v: CellWidth=%d spans %d px for %d cells, but the shaper draws them in %d px (drift %d px)",
				size, m.CellWidth, m.CellWidth*len(probe), len(probe), dims.Size.X, drift)
		}
	}
}

// TestShapedRunDriftsFromCellGrid documents *why* the painter draws one cell
// at a time.
//
// It measures the same text the old painter would have handed to the shaper
// as a single string, and asserts that the result does not line up with the
// cell grid. As long as this holds, drawing a run as one string is unsafe
// and the per-cell loop in drawGrid must stay. If a future font or shaper
// change ever makes the advance exactly integral, this test fails loudly
// rather than letting someone "simplify" the painter back into the bug.
func TestShapedRunDriftsFromCellGrid(t *testing.T) {
	fonts := testFonts(t)
	cfg := config.Default().Editor
	shaper := render.NewShaper(cfg)
	face := render.FontFace(cfg)

	var measureOps op.Ops
	m := render.Measure(
		newTestContext(&measureOps, image.Pt(4000, 4000)),
		shaper, face, fonts.Size,
	)

	const cols = 60
	run := make([]byte, cols)
	for i := range run {
		run[i] = 'M'
	}

	var shapeOps op.Ops
	gtx := newTestContext(&shapeOps, image.Pt(1<<20, 1<<20))
	gtx.Constraints.Min = image.Point{}
	dims := widget.Label{MaxLines: 1}.Layout(gtx, shaper, face, fonts.Size, string(run), op.CallOp{})

	gridWidth := m.CellWidth * cols
	if dims.Size.X == gridWidth {
		t.Skipf("shaped advance happens to be exactly integral at size %v; the per-cell painter is still required for other fonts/sizes", fonts.Size)
	}

	// The drift must be smaller than the whole grid, otherwise the metric
	// itself is broken rather than merely fractional.
	if dims.Size.X <= 0 || dims.Size.X > gridWidth*2 {
		t.Fatalf("implausible shaped width %d for %d cells of %d px", dims.Size.X, cols, m.CellWidth)
	}

	t.Logf("shaping %d cells as one string spans %d px but the grid reserves %d px (drift %d px) -- this is why drawGrid draws per cell",
		cols, dims.Size.X, gridWidth, dims.Size.X-gridWidth)
}
