package editorapp

import (
	"image"
	"os/exec"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/unit"

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

// waitBar applies Nvim's redraws until the single scrollbar satisfies ok.
// Row views arrive as notifications on their own goroutine, so it also
// rechecks on a short tick rather than only on redraws.
func waitBar(t *testing.T, a *App, what string, ok func(render.Scrollbar) bool) render.Scrollbar {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		if bars := a.scrollbars(a.state.Snapshot()); len(bars) == 1 && ok(bars[0]) {
			return bars[0]
		}
		select {
		case batch := <-a.proc.Redraw:
			a.state.Apply(batch)
		case <-time.After(20 * time.Millisecond):
		case <-deadline:
			t.Fatalf("timed out waiting for %s (bars %+v)", what, a.scrollbars(a.state.Snapshot()))
		}
	}
}

// TestScrollbarCoversOneLongWrappedLine checks a single line wrapped over
// more rows than the window has gets a bar (by win_viewport's line counts
// it fits), that the thumb follows scrolling within the line, and that a
// press on the track scrolls part-way through it.
func TestScrollbarCoversOneLongWrappedLine(t *testing.T) {
	for _, smooth := range []bool{true, false} {
		a := scrollbarApp(t, true)
		var has int
		if err := a.proc.Nvim.Eval("has('nvim-0.10')", &has); err != nil || has == 0 {
			t.Skip("row-based bars need Nvim 0.10+ (nvim_win_text_height, smoothscroll)")
		}
		a.installRowViews()
		opt := "nosmoothscroll"
		if smooth {
			opt = "smoothscroll"
		}
		if err := a.proc.Nvim.Command("silent %delete _ | call setline(1, repeat('a', 20000)) | set " + opt); err != nil {
			t.Fatal(err)
		}
		b := waitBar(t, a, "a row-based bar", func(b render.Scrollbar) bool {
			return b.Rows && b.LineCount == 250 && b.Thumb.Min.Y == b.Track.Min.Y
		})

		mid := image.Pt(b.Track.Min.X+1, b.Track.Min.Y+b.Track.Dy()/2)
		snap := a.state.Snapshot()
		a.onScrollbarPointer(leftEvent(pointer.Press, mid), snap)
		a.onScrollbarPointer(leftEvent(pointer.Release, mid), snap)
		b = waitBar(t, a, "the thumb to follow the press", func(b render.Scrollbar) bool {
			return b.Thumb.Min.Y > b.Track.Min.Y && b.Thumb.Max.Y < b.Track.Max.Y
		})
		if c := (b.Thumb.Min.Y + b.Thumb.Max.Y) / 2; c < mid.Y-16 || c > mid.Y+16 {
			t.Errorf("smoothscroll=%v: thumb %v not centred on the press at y=%d", smooth, b.Thumb, mid.Y)
		}
		var skip int
		if err := a.proc.Nvim.Eval("winsaveview().skipcol", &skip); err != nil {
			t.Fatal(err)
		}
		if skip <= 0 {
			t.Errorf("smoothscroll=%v: skipcol = %d after a mid-track press, want > 0", smooth, skip)
		}
	}
}

// TestScrollbarWidthIsInPoints checks scrollbar_width scales with the
// display like the font does, and that unset keeps the cell-based width.
func TestScrollbarWidthIsInPoints(t *testing.T) {
	a := scrollbarApp(t, true)
	a.fonts.Metrics.CellWidth = 17
	for _, tc := range []struct {
		points float64
		scale  float32
		want   int
	}{
		{0, 2, 11},
		{3, 1, 3},
		{3, 2, 6},
		{20, 2, 17},
	} {
		a.cfg.Editor.ScrollbarWidth = tc.points
		a.metric = unit.Metric{PxPerDp: tc.scale, PxPerSp: tc.scale}
		bars := a.scrollbars(a.state.Snapshot())
		if len(bars) != 1 || bars[0].Track.Dx() != tc.want {
			t.Errorf("scrollbar_width=%v at %vx: bars %+v, want one %d px wide", tc.points, tc.scale, bars, tc.want)
		}
	}
}

// TestScrollbarGetsItsOwnColumn checks the bar no longer covers text: with
// the setting on, the grid is one column short of the window and the bar
// sits in that spare column; with it off, the grid uses the full width.
func TestScrollbarGetsItsOwnColumn(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		a := scrollbarApp(t, enabled)
		a.syncSize(image.Pt(100*8, 24*16))
		want := 100
		if enabled {
			want = 99
		}
		if a.cols != want {
			t.Fatalf("scrollbar=%v: grid is %d columns, want %d", enabled, a.cols, want)
		}
		waitTopline(t, a, "resize", func(uistate.Viewport) bool {
			for _, p := range a.state.Snapshot().Windows {
				if p.GridID == 2 {
					return p.Width == want
				}
			}
			return false
		})
		bars := a.scrollbars(a.state.Snapshot())
		if !enabled {
			if bars != nil {
				t.Fatalf("scrollbars = %+v with the setting off", bars)
			}
			continue
		}
		if len(bars) != 1 {
			t.Fatalf("got %d scrollbars, want 1", len(bars))
		}
		if b := bars[0]; b.Track.Min.X < want*8 || b.Track.Max.X > 100*8 {
			t.Errorf("track %v should lie in the spare column x %d..%d", b.Track, want*8, 100*8)
		}
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
