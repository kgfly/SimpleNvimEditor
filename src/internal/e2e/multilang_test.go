package e2e

import (
	"image"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
	"github.com/kgfly/SimpleNvimEditor/internal/input"
	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
	"github.com/kgfly/SimpleNvimEditor/internal/render"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// TestMultiLanguageWithRealConfig renders a multi-script line using the
// user's actual on-disk config through the whole live stack, and writes a
// PNG for visual confirmation.
func TestMultiLanguageWithRealConfig(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("config FontFamily=%q system=%v", cfg.Editor.FontFamily, cfg.Editor.UseSystemFonts)
	t.Logf("resolved typeface = %q", render.FontFace(cfg.Editor).Typeface)

	px := image.Pt(1400, 200)
	scale := float32(2)

	var mops op.Ops
	mgtx := layout.Context{
		Ops:         &mops,
		Metric:      unit.Metric{PxPerDp: scale, PxPerSp: scale},
		Constraints: layout.Exact(px),
	}
	fonts := render.Fonts{
		Shaper: render.NewShaper(),
		Face:   render.FontFace(cfg.Editor),
		Size:   unit.Sp(cfg.Editor.FontSize),
	}
	fonts.Metrics = render.Measure(mgtx, fonts.Shaper, fonts.Face, fonts.Size)

	cols := px.X / fonts.Metrics.CellWidth
	rows := px.Y / fonts.Metrics.CellHeight

	proc, err := nvimproc.Spawn("nvim", []string{"--clean"}, nil, cols, rows)
	if err != nil {
		t.Skipf("nvim unavailable: %v", err)
	}
	defer proc.RequestQuit()

	st := uistate.New()
	go func() {
		for batch := range proc.Redraw {
			st.Apply(batch)
		}
	}()

	proc.Input("i")
	proc.Input(input.EncodeText(sample))

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if strings.Contains(bufferRow0(st.Snapshot()), "\u4f60\u597d") {
			break
		}
	}
	snap := st.Snapshot()

	w, err := headless.NewWindow(px.X, px.Y)
	if err != nil {
		t.Skipf("headless GPU unavailable: %v", err)
	}
	defer w.Release()

	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      unit.Metric{PxPerDp: scale, PxPerSp: scale},
		Constraints: layout.Exact(px),
	}
	render.Frame(gtx, fonts, snap)
	if err := w.Frame(&ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: px})
	if err := w.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create("/tmp/multilang_fixed.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	t.Log("wrote /tmp/multilang_fixed.png")
}
