package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// waitForViewport applies redraw batches until some window's viewport
// satisfies ok, and returns it.
func waitForViewport(t *testing.T, proc *nvimproc.Process, s *uistate.State, what string, ok func(uistate.Viewport) bool) uistate.Viewport {
	t.Helper()
	find := func() (uistate.Viewport, bool) {
		for _, v := range s.Snapshot().Viewports {
			if ok(v) {
				return v, true
			}
		}
		return uistate.Viewport{}, false
	}
	if v, found := find(); found {
		return v
	}
	deadline := time.After(drainTimeout)
	for {
		select {
		case batch := <-proc.Redraw:
			s.Apply(batch)
			if v, found := find(); found {
				return v
			}
		case <-deadline:
			t.Fatalf("timed out after %s waiting for viewport: %s (have %+v)", drainTimeout, what, s.Snapshot().Viewports)
		}
	}
}

// TestScrollWindowScrollsBuffer covers the scrollbar's round trip: Nvim
// reports the window's viewport, and ScrollWindow moves it, including all
// the way to the bottom and back to the top.
func TestScrollWindowScrollsBuffer(t *testing.T) {
	lines := make([]string, 300)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	file := filepath.Join(t.TempDir(), "long.txt")
	if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	proc := spawnIsolated(t, file, 80, 24)
	s := uistate.New()

	v := waitForViewport(t, proc, s, "300-line file at the top", func(v uistate.Viewport) bool {
		return v.LineCount == 300 && v.Topline == 0
	})
	if v.Win == 0 {
		t.Fatal("viewport carries no window handle")
	}

	proc.ScrollWindow(v.Win, 100, false)
	waitForViewport(t, proc, s, "topline 100", func(v uistate.Viewport) bool { return v.Topline == 100 })

	proc.ScrollWindow(v.Win, 300, false)
	waitForViewport(t, proc, s, "last line visible", func(v uistate.Viewport) bool {
		return v.Botline >= 300 && v.Topline > 250
	})

	proc.ScrollWindow(v.Win, 0, false)
	waitForViewport(t, proc, s, "back at the top", func(v uistate.Viewport) bool { return v.Topline == 0 })
}

// TestScrollWindowScrollsTerminalInTerminalMode is why scrollbars are drawn
// natively rather than by a plugin: Nvim reports terminal viewports too.
// Terminal mode pins the view to the output, so a scroll has to leave it.
func TestScrollWindowScrollsTerminalInTerminalMode(t *testing.T) {
	proc := spawnIsolated(t, filepath.Join(t.TempDir(), "empty.txt"), 80, 24)
	s := uistate.New()
	drainUntilFlush(t, proc, s)

	if err := proc.Nvim.Command(`terminal sh -c 'seq 1 300; sleep 60'`); err != nil {
		t.Fatal(err)
	}
	if err := proc.Nvim.Command("startinsert"); err != nil {
		t.Fatal(err)
	}
	v := waitForViewport(t, proc, s, "terminal scrolled to its output", func(v uistate.Viewport) bool {
		return v.LineCount > 250 && v.Topline > 250
	})

	proc.ScrollWindow(v.Win, 50, false)
	waitForViewport(t, proc, s, "terminal topline 50", func(v uistate.Viewport) bool { return v.Topline == 50 })

	mode, err := proc.Nvim.Mode()
	if err != nil {
		t.Fatal(err)
	}
	if mode.Mode == "t" {
		t.Errorf("still in Terminal mode after scrolling; the view would snap back to the output")
	}
}
