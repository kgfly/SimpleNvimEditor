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

// TestCJKEndToEnd drives the whole stack the way the real app does: a live
// nvim, the user's actual config, real highlight groups from
// default_colors_set, and the real renderer. The PNG it writes is what the
// user would actually see.
func TestCJKEndToEnd(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("config: FontFamily=%q size=%v system=%v",
		cfg.Editor.FontFamily, cfg.Editor.FontSize, cfg.Editor.UseSystemFonts)

	px := image.Pt(900, 300)
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
	t.Logf("cell = %dx%d", fonts.Metrics.CellWidth, fonts.Metrics.CellHeight)

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

	const want = "中文字测试"
	proc.Input("i")
	proc.Input(input.EncodeText(want))

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if strings.Contains(bufferRow0(st.Snapshot()), want) {
			break
		}
	}

	snap := st.Snapshot()
	got := bufferRow0(snap)
	t.Logf("buffer row 0 = %q", got)
	fg, bg := snap.Highlight.DefaultColors()
	t.Logf("default colors fg=%v bg=%v", fg, bg)

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
	f, err := os.Create("/tmp/cjk_e2e.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	t.Log("wrote /tmp/cjk_e2e.png")

	if !strings.Contains(got, want) {
		t.Errorf("buffer = %q, want %q", got, want)
	}
}

func bufferRow0(snap uistate.Snapshot) string {
	best := -1
	for id := range snap.Grids {
		if id == 1 {
			continue
		}
		if best == -1 || id < best {
			best = id
		}
	}
	if best == -1 {
		return ""
	}
	gv := snap.Grids[best]
	if len(gv.Data) == 0 {
		return ""
	}
	var b strings.Builder
	for _, c := range gv.Data[0] {
		b.WriteString(c.Text)
	}
	return strings.TrimRight(b.String(), " ")
}
