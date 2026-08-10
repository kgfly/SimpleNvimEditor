package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

const drainTimeout = 8 * time.Second

// spawnIsolated starts a real Nvim child process pointed at file, with
// `-u NONE -n` so it ignores the host's real init.lua/vimrc and swapfiles —
// the test must behave the same on a bare CI box as on a dev machine with a
// heavy personal Neovim config.
func spawnIsolated(t *testing.T, file string, cols, rows int) *nvimproc.Process {
	t.Helper()
	nvimPath := requireNvim(t)

	proc, err := nvimproc.Spawn(nvimPath, []string{"-u", "NONE", "-n"}, []string{file}, cols, rows)
	if err != nil {
		t.Fatalf("nvimproc.Spawn: %v", err)
	}
	t.Cleanup(func() {
		proc.RequestQuit()
		select {
		case <-proc.Exited:
			return
		case <-time.After(2 * time.Second):
		}
		// Graceful quit didn't finish in time (e.g. a modified buffer we
		// forgot to clear left Nvim blocked on a "save changes?" prompt).
		// Force-close the RPC connection so the child process sees EOF and
		// exits; go-client's Close hard-kills it after a further timeout as
		// a last resort. Either way, this test process must never leak an
		// orphaned nvim.
		t.Logf("nvim did not exit gracefully after RequestQuit; force-closing")
		_ = proc.Nvim.Close()
		<-proc.Exited
	})
	return proc
}

// containsLine reports whether any row of any grid in the current snapshot
// trims (dropping right-padding spaces) to exactly want. Note: with
// ext_multigrid enabled (which this client always requests), even a single,
// unsplit window's buffer content is *not* drawn into grid 1 — grid 1 is
// reserved for the outer chrome (message/cmdline area), and the window gets
// its own grid id (2, in practice, but this isn't a protocol guarantee).
// Searching every grid is what a real renderer effectively does too (see
// render.Frame, which draws grid 1 plus every placed window grid), so this
// matches what a user would actually see on screen rather than assuming a
// specific internal grid id.
func containsLine(s *uistate.State, want string) bool {
	for _, g := range s.Snapshot().Grids {
		for _, row := range g.Data {
			if rowText(row) == want {
				return true
			}
		}
	}
	return false
}

func rowText(row []uistate.Cell) string {
	var b strings.Builder
	for _, cell := range row {
		b.WriteString(cell.Text)
	}
	return strings.TrimRight(b.String(), " ")
}

// drainUntilFlush applies redraw batches from proc into s until one of them
// contains a flush event (meaning Nvim has finished this round of updates
// and the screen is consistent), or the timeout elapses. The *first* flush
// isn't guaranteed to already contain final buffer content (Nvim can paint
// several intermediate frames — option_set, hl_attr_define, ... — before
// the buffer's own grid_line events arrive) — callers that care about
// specific content should use waitForLine instead.
func drainUntilFlush(t *testing.T, proc *nvimproc.Process, s *uistate.State) {
	t.Helper()
	deadline := time.After(drainTimeout)
	for {
		select {
		case batch := <-proc.Redraw:
			if s.Apply(batch) {
				return
			}
		case <-deadline:
			t.Fatalf("timed out after %s waiting for a flush event", drainTimeout)
		}
	}
}

// waitForLine polls proc.Redraw, applying every batch to s, until want
// appears as a full (trimmed) row somewhere in the rendered grids, or the
// timeout elapses.
func waitForLine(t *testing.T, proc *nvimproc.Process, s *uistate.State, want string) {
	t.Helper()
	if containsLine(s, want) {
		return
	}
	deadline := time.After(drainTimeout)
	for {
		select {
		case batch := <-proc.Redraw:
			s.Apply(batch)
			if containsLine(s, want) {
				return
			}
		case <-deadline:
			t.Fatalf("timed out after %s waiting for a rendered line %q", drainTimeout, want)
		}
	}
}

func TestSpawnAttachesAndRendersFileContent(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(file, []byte("hello integration test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	proc := spawnIsolated(t, file, 40, 10)
	s := uistate.New()
	waitForLine(t, proc, s, "hello integration test")
}

func TestInputIsReflectedInBothOurGridAndRealNvimBuffer(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "edit.txt")
	if err := os.WriteFile(file, []byte("start\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	proc := spawnIsolated(t, file, 40, 10)
	s := uistate.New()
	waitForLine(t, proc, s, "start")

	// Go to end of line, append text, return to Normal.
	proc.Input("A")
	proc.Input(" edited")
	proc.Input("<Esc>")

	waitForLine(t, proc, s, "start edited")

	// Cross-check against the real Nvim buffer directly, independent of our
	// own redraw-derived mirror, to make sure the edit actually landed and
	// isn't just an artifact of how we decode grid_line.
	line, err := proc.Nvim.CommandOutput("echo getline(1)")
	if err != nil {
		t.Fatalf("CommandOutput: %v", err)
	}
	if strings.TrimSpace(line) != "start edited" {
		t.Fatalf("Nvim buffer line 1 = %q, want %q", strings.TrimSpace(line), "start edited")
	}

	// The buffer is now modified; clear that flag before the standard
	// spawnIsolated cleanup runs `confirm qa`, which would otherwise block
	// on Nvim's "save changes?" prompt and leak the process for the
	// duration of the cleanup timeout.
	if err := proc.Nvim.Command("set nomodified"); err != nil {
		t.Fatalf("set nomodified: %v", err)
	}
}

func TestResizeChangesNvimGridDimensions(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "resize.txt")
	if err := os.WriteFile(file, []byte("x\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	proc := spawnIsolated(t, file, 40, 10)
	s := uistate.New()
	drainUntilFlush(t, proc, s)

	proc.Resize(60, 15)

	// Resize (like Input) is queued asynchronously; poll Nvim's own &columns
	// option, which is the ground truth for what TryResizeUI actually did.
	deadline := time.After(drainTimeout)
	for {
		out, err := proc.Nvim.CommandOutput("echo &columns")
		if err != nil {
			t.Fatalf("CommandOutput: %v", err)
		}
		if strings.TrimSpace(out) == "60" {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("&columns = %q after timeout, want \"60\"", strings.TrimSpace(out))
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestInputMouseDoesNotBreakTheSession(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "mouse.txt")
	content := "line one\nline two\nline three\n"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	proc := spawnIsolated(t, file, 40, 10)
	s := uistate.New()
	drainUntilFlush(t, proc, s)

	proc.InputMouse("left", "press", "", 1, 1, 2)
	proc.InputMouse("left", "release", "", 1, 1, 2)

	// The session should still be alive and responsive afterwards.
	out, err := proc.Nvim.CommandOutput("echo 1+1")
	if err != nil {
		t.Fatalf("CommandOutput after mouse input: %v", err)
	}
	if strings.TrimSpace(out) != "2" {
		t.Fatalf("CommandOutput echo 1+1 = %q, want \"2\"", strings.TrimSpace(out))
	}
}

func TestRequestQuitExitsCleanly(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "quit.txt")
	if err := os.WriteFile(file, []byte("bye\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	proc := spawnIsolated(t, file, 40, 10)
	s := uistate.New()
	drainUntilFlush(t, proc, s)

	proc.RequestQuit()
	select {
	case <-proc.Exited:
	case <-time.After(drainTimeout):
		t.Fatalf("nvim did not exit within %s of RequestQuit", drainTimeout)
	}
	if proc.ServeErr != nil {
		t.Fatalf("ServeErr = %v, want nil after a clean quit", proc.ServeErr)
	}
}
