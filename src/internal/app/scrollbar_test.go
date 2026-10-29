package editorapp

import (
	"image"
	"os/exec"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
	"github.com/kgfly/SimpleNvimEditor/internal/render"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// scrollbarApp starts a real Nvim showing a 300-line buffer and returns an
// App wired to it with the scrollbar setting as given.
func scrollbarApp(t *testing.T, enabled bool) *App {
	t.Helper()
	path, err := exec.LookPath("nvim")
	if err != nil {
		t.Skip("nvim not found on PATH")
	}
	proc, err := nvimproc.Spawn(path, []string{"-u", "NONE", "-n"}, nil, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = proc.Nvim.Close()
		<-proc.Exited
	})
	cfg := config.Default()
	cfg.Editor.Scrollbar = enabled
	a := &App{cfg: cfg, proc: proc, state: uistate.New()}
	a.fonts.Metrics = render.Metrics{CellWidth: 8, CellHeight: 16, Baseline: 12}
	if err := proc.Nvim.Command("call setline(1, map(range(1, 300), 'string(v:val)'))"); err != nil {
		t.Fatal(err)
	}
	waitTopline(t, a, "300-line buffer", func(v uistate.Viewport) bool { return v.LineCount == 300 })
	return a
}

// waitTopline applies Nvim's redraws until a viewport satisfies ok.
func waitTopline(t *testing.T, a *App, what string, ok func(uistate.Viewport) bool) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		for _, v := range a.state.Snapshot().Viewports {
			if ok(v) {
				return
			}
		}
		select {
		case batch := <-a.proc.Redraw:
			a.state.Apply(batch)
		case <-deadline:
			t.Fatalf("timed out waiting for %s (have %+v)", what, a.state.Snapshot().Viewports)
		}
	}
}

func leftEvent(kind pointer.Kind, p image.Point) pointer.Event {
	e := pointer.Event{Kind: kind, Position: f32.Pt(float32(p.X)+0.5, float32(p.Y)+0.5)}
	if kind != pointer.Release {
		e.Buttons = pointer.ButtonPrimary
	}
	return e
}

// TestScrollbarClickJumpsAndDragScrolls drives the pointer path the way
// Gio delivers it: a press on the empty track jumps there, and pressing
// the thumb then dragging moves the view with it.
func TestScrollbarClickJumpsAndDragScrolls(t *testing.T) {
	a := scrollbarApp(t, true)
	snap := a.state.Snapshot()
	bars := a.scrollbars(snap)
	if len(bars) != 1 {
		t.Fatalf("got %d scrollbars, want 1", len(bars))
	}
	b := bars[0]
	if b.Thumb.Min.Y != b.Track.Min.Y {
		t.Fatalf("thumb %v should start at the top of track %v", b.Thumb, b.Track)
	}

	bottom := image.Pt(b.Track.Min.X+1, b.Track.Max.Y-1)
	if !a.onScrollbarPointer(leftEvent(pointer.Press, bottom), snap) {
		t.Fatal("press on the track was not consumed")
	}
	if !a.onScrollbarPointer(leftEvent(pointer.Release, bottom), snap) {
		t.Fatal("release after a track press was not consumed")
	}
	waitTopline(t, a, "jump to the end", func(v uistate.Viewport) bool { return v.Botline >= 300 })

	snap = a.state.Snapshot()
	b = a.scrollbars(snap)[0]
	grab := image.Pt(b.Thumb.Min.X+1, b.Thumb.Min.Y+2)
	a.onScrollbarPointer(leftEvent(pointer.Press, grab), snap)
	if a.scrollDrag == nil {
		t.Fatal("press on the thumb did not start a drag")
	}
	mid := b.Track.Min.Y + (b.Track.Dy()-b.Thumb.Dy())/2 + 2
	a.onScrollbarPointer(leftEvent(pointer.Drag, image.Pt(grab.X, mid)), snap)
	waitTopline(t, a, "drag to the middle", func(v uistate.Viewport) bool { return v.Topline > 100 && v.Topline < 180 })
	a.onScrollbarPointer(leftEvent(pointer.Drag, image.Pt(grab.X, -50)), snap)
	a.onScrollbarPointer(leftEvent(pointer.Release, image.Pt(grab.X, -50)), snap)
	waitTopline(t, a, "drag back to the top", func(v uistate.Viewport) bool { return v.Topline == 0 })
	if a.scrollDrag != nil {
		t.Error("drag still active after release")
	}

	text := image.Pt(10, 10)
	if a.onScrollbarPointer(leftEvent(pointer.Press, text), a.state.Snapshot()) {
		t.Error("a click in the text was swallowed by the scrollbar")
	}
}

// TestScrollbarOffLeavesClicksToNvim checks the default: no bar, and a
// click at the right edge goes to Nvim like any other.
func TestScrollbarOffLeavesClicksToNvim(t *testing.T) {
	a := scrollbarApp(t, false)
	snap := a.state.Snapshot()
	if bars := a.scrollbars(snap); bars != nil {
		t.Fatalf("scrollbars = %+v with the setting off", bars)
	}
	edge := image.Pt(80*8-2, 16*5)
	if a.onScrollbarPointer(leftEvent(pointer.Press, edge), snap) {
		t.Error("press at the right edge was consumed with the setting off")
	}
}
