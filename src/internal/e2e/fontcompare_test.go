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

// sample deliberately mixes scripts so the screenshot shows whether the fix
// is CJK-specific or genuinely multi-language.
const sample = "你知道吗 test 好中文 | kana あいう | 한글 | Привет | Ωμέγα"

// renderWith drives a live nvim with the given font family and writes a PNG.
func renderWith(t *testing.T, name, family string) {
	t.Helper()

	ed := config.EditorConfig{FontSize: 12, UseSystemFonts: true, FontFamily: family}
	if family == "" {
		ed = config.Default().Editor
	}

	px := image.Pt(1400, 220)
	scale := float32(2)

	var mops op.Ops
	mgtx := layout.Context{
		Ops:         &mops,
		Metric:      unit.Metric{PxPerDp: scale, PxPerSp: scale},
		Constraints: layout.Exact(px),
	}
	fonts := render.Fonts{
		Shaper: render.NewShaper(),
		Face:   render.FontFace(ed),
		Size:   unit.Sp(ed.FontSize),
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
	path := "/tmp/font_" + name + ".png"
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	t.Logf("family=%q cell=%dx%d -> %s", family, fonts.Metrics.CellWidth, fonts.Metrics.CellHeight, path)
}

func TestFontCompareCurrent(t *testing.T) {
	renderWith(t, "current", "Hack Nerd Font Mono Bold")
}

func TestFontCompareChain(t *testing.T) {
	renderWith(t, "chain", "Hack Nerd Font Mono, Menlo, PingFang SC, Hiragino Sans, Apple SD Gothic Neo, Arial Unicode MS")
}

func TestChainNoBold(t *testing.T) {
	renderWith(t, "nobold", "Hack Nerd Font Mono, Menlo, PingFang SC, Hiragino Sans, Apple SD Gothic Neo, Arial Unicode MS")
}

func TestChainCJKFirst(t *testing.T) {
	renderWith(t, "cjkfirst", "Hack Nerd Font Mono, PingFang SC, Hiragino Sans, Apple SD Gothic Neo")
}

func TestChainBoldNoArial(t *testing.T) {
	renderWith(t, "boldnoarial", "Hack Nerd Font Mono Bold, PingFang SC, Hiragino Sans, Apple SD Gothic Neo")
}
