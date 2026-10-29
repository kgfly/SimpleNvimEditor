package editorapp

import (
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/kgfly/SimpleNvimEditor/internal/render"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

func TestZZDiagTextBuffer(t *testing.T) {
	px := image.Pt(1400, 800)
	a := newMeasuredApp()
	a.cfg.Editor.Scrollbar = true
	a.state = uistate.New()
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(px)}
	a.syncMetrics(gtx)
	m := a.fonts.Metrics
	cols, rows := px.X/m.CellWidth-1, px.Y/m.CellHeight

	path, _ := exec.LookPath("nvim")
	file, _ := filepath.Abs("app.go")
	proc, err := nvimproc_Spawn(path, file, cols, rows)
	if err != nil {
		t.Fatal(err)
	}
	defer proc.Nvim.Close()
	a.proc = proc
	cmd := os.Getenv("ZZCMD")
	deadline := time.After(5 * time.Second)
	go func() {
		time.Sleep(2 * time.Second)
		if cmd != "" {
			_ = proc.Nvim.Command(cmd)
		}
	}()
loop:
	for {
		select {
		case b := <-proc.Redraw:
			a.state.Apply(b)
		case <-deadline:
			break loop
		}
	}
	snap := a.state.Snapshot()
	t.Logf("windows %+v", snap.Windows)
	t.Logf("viewports %+v", snap.Viewports)
	bars := a.scrollbars(snap)
	t.Logf("bars %+v", bars)

	w, err := headless.NewWindow(px.X, px.Y)
	if err != nil {
		t.Skip(err)
	}
	defer w.Release()
	ops.Reset()
	render.FrameWith(gtx, a.fonts, snap, render.FrameOptions{Scrollbars: bars})
	if err := w.Frame(&ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: px})
	if err := w.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	for _, b := range bars {
		t.Logf("grid %d thumb px %v track px %v", b.GridID, img.At(b.Thumb.Min.X+2, b.Thumb.Min.Y+5), img.At(b.Track.Min.X+2, b.Track.Max.Y-5))
	}
	f, _ := os.Create("/tmp/zzdiag.png")
	_ = png.Encode(f, img)
	f.Close()
}
