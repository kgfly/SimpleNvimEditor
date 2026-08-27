package editorapp

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
	"github.com/kgfly/SimpleNvimEditor/internal/render"
)

// frameAt builds the layout.Context Gio would hand us for a window being
// rendered at the given pixel density. scale is macOS's backingScaleFactor:
// 2 on a Retina panel, 1 on a typical external monitor.
func frameAt(ops *op.Ops, scale float32) layout.Context {
	return layout.Context{
		Ops:         ops,
		Metric:      unit.Metric{PxPerDp: scale, PxPerSp: scale},
		Constraints: layout.Exact(image.Pt(1000, 650)),
	}
}

// newMeasuredApp returns an App with a real shaper, ready to lay out.
func newMeasuredApp() *App {
	cfg := config.Default()
	a := New(cfg, nil, Options{})
	a.fonts = render.Fonts{
		Shaper: render.NewShaper(cfg.Editor),
		Face:   render.FontFace(cfg.Editor),
		Size:   unit.Sp(cfg.Editor.FontSize),
	}
	return a
}

// TestMetricsFollowDisplayScale is the regression test for text becoming
// distorted after dragging the window to a monitor with a different
// resolution, and staying distorted across maximize/minimize.
//
// Cell size is derived from the shaped size of the font, and the shaper
// converts sp to pixels via gtx.Metric.PxPerSp. Measuring once and caching
// forever leaves the grid laid out for the monitor the window started on,
// so glyphs are drawn at the new scale but positioned on cells sized for
// the old one. Maximizing does not help: it changes the window's size, not
// its pixel density, so the stale metrics survive.
func TestMetricsFollowDisplayScale(t *testing.T) {
	a := newMeasuredApp()

	var ops1x op.Ops
	a.syncMetrics(frameAt(&ops1x, 1))
	at1x := a.fonts.Metrics
	if at1x.CellWidth == 0 || at1x.CellHeight == 0 {
		t.Fatalf("metrics not measured at 1x: %+v", at1x)
	}

	// Drag to a 2x display.
	var ops2x op.Ops
	a.syncMetrics(frameAt(&ops2x, 2))
	at2x := a.fonts.Metrics

	if at2x == at1x {
		t.Fatalf("metrics unchanged after a scale change (%+v); the grid is still sized for the old monitor", at1x)
	}
	if at2x.CellWidth <= at1x.CellWidth || at2x.CellHeight <= at1x.CellHeight {
		t.Errorf("cells did not grow with the display scale: 1x=%+v 2x=%+v", at1x, at2x)
	}

	// Dragging back must restore the original metrics exactly, otherwise
	// the distortion would merely be different rather than fixed.
	var back op.Ops
	a.syncMetrics(frameAt(&back, 1))
	if a.fonts.Metrics != at1x {
		t.Errorf("returning to 1x gave %+v, want the original %+v", a.fonts.Metrics, at1x)
	}
}

// TestMetricsStableWithinOneDisplay guards the other half: re-measuring is
// keyed on the density, so ordinary frames (including maximize/minimize,
// which change size but not scale) must not re-shape the probe string.
func TestMetricsStableWithinOneDisplay(t *testing.T) {
	a := newMeasuredApp()

	var ops op.Ops
	a.syncMetrics(frameAt(&ops, 2))
	first := a.fonts.Metrics

	for i := 0; i < 3; i++ {
		gtx := frameAt(&ops, 2)
		// Maximize: a bigger window at the same pixel density.
		gtx.Constraints = layout.Exact(image.Pt(2560, 1440))
		a.syncMetrics(gtx)
	}

	if a.fonts.Metrics != first {
		t.Errorf("metrics drifted without a scale change: %+v, want %+v", a.fonts.Metrics, first)
	}
}

// TestGridResizesAfterScaleChange proves the fix reaches Nvim. Cell size
// determines how many columns and rows fit, so a scale change must also
// re-derive the grid dimensions; otherwise Nvim keeps drawing to a grid
// that no longer matches the window.
func TestGridResizesAfterScaleChange(t *testing.T) {
	a := newMeasuredApp()
	const windowPx = 1000

	var ops1x op.Ops
	a.syncMetrics(frameAt(&ops1x, 1))
	cols1x := windowPx / a.fonts.Metrics.CellWidth

	var ops2x op.Ops
	a.syncMetrics(frameAt(&ops2x, 2))
	cols2x := windowPx / a.fonts.Metrics.CellWidth

	if cols2x >= cols1x {
		t.Errorf("a %dpx window fits %d cols at 1x and %d at 2x; cells must get bigger, so columns must get fewer",
			windowPx, cols1x, cols2x)
	}
}
