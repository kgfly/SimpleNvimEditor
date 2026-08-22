// Package e2e_test drives the actual `simplenvim` GUI binary end-to-end: builds
// it, launches it under a virtual X11 display (Xvfb), synthesizes real
// keyboard/window-system events with xdotool, and verifies the rendered
// output by decoding a screenshot — mirroring exactly how this project was
// manually verified during development.
//
// This tier only runs on Linux with Xvfb, xdotool, ImageMagick's `import`,
// and `nvim` all present on PATH; otherwise every test skips itself so the
// suite stays runnable on machines that don't have a full X11 toolchain
// (e.g. macOS/Windows dev machines, or minimal CI images).
//
// Run with: go test ./test/e2e/...
package e2e_test

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const windowTitle = "SimpleNvimEditor"

// requireE2ETools skips the test unless every external tool this tier
// depends on is available.
func requireE2ETools(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("e2e GUI tests only run on Linux (Xvfb is X11-specific)")
	}
	for _, tool := range []string{"Xvfb", "xdotool", "import", "nvim", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("required tool %q not found on PATH; skipping e2e test", tool)
		}
	}
}

// buildSimplenvim compiles the real cmd/simplenvim binary into a temp
// directory and returns its path.
func buildSimplenvim(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "simplenvim")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/simplenvim")
	cmd.Dir = repoSrcDir(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./cmd/simplenvim: %v\n%s", err, out)
	}
	return bin
}

// repoSrcDir returns the src/ module root, derived from this test file's
// own location (test/e2e -> src) rather than assuming a working directory.
func repoSrcDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	// go test runs with the package directory as cwd: src/test/e2e.
	return filepath.Join(wd, "..", "..")
}

// startXvfb launches Xvfb on the first free display it finds and returns a
// ":N" display string plus a cleanup function.
func startXvfb(t *testing.T) string {
	t.Helper()
	for n := 100; n < 200; n++ {
		lock := fmt.Sprintf("/tmp/.X%d-lock", n)
		if _, err := os.Stat(lock); err == nil {
			continue // display already in use
		}
		display := ":" + strconv.Itoa(n)
		cmd := exec.Command("Xvfb", display, "-screen", "0", "1024x768x24")
		if err := cmd.Start(); err != nil {
			continue
		}
		// Wait for the X server to be fully ready: the lock file appears
		// quickly but the unix socket is what clients actually connect to.
		ready := false
		sock := fmt.Sprintf("/tmp/.X11-unix/X%d", n)
		for i := 0; i < 150; i++ {
			if _, errL := os.Stat(lock); errL == nil {
				if _, errS := os.Stat(sock); errS == nil {
					ready = true
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		if !ready {
			_ = cmd.Process.Kill()
			continue
		}
		t.Cleanup(func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		})
		return display
	}
	t.Fatalf("could not find a free X display for Xvfb in range :100-:199")
	return ""
}

// waitForWindow polls xdotool until a window with our title appears (or the
// timeout elapses) and returns its window id.
func waitForWindow(t *testing.T, display string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		cmd := exec.Command("xdotool", "search", "--name", windowTitle)
		cmd.Env = append(os.Environ(), "DISPLAY="+display)
		out, err := cmd.Output()
		if err == nil {
			if id := strings.TrimSpace(string(out)); id != "" {
				return strings.Split(id, "\n")[0]
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("window %q did not appear within %s", windowTitle, timeout)
	return ""
}

// screenshotWindow captures window id into a new PNG file and returns the
// decoded image.
func screenshotWindow(t *testing.T, display, windowID string) image.Image {
	t.Helper()
	path := filepath.Join(t.TempDir(), "shot.png")
	cmd := exec.Command("import", "-window", windowID, path)
	cmd.Env = append(os.Environ(), "DISPLAY="+display)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("import screenshot: %v\n%s", err, out)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open screenshot: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode screenshot: %v", err)
	}
	return img
}

// distinctColorCount samples a grid of pixels across img and returns how
// many distinct colors it saw — a cheap way to tell "real content was
// rendered" from "the window is just a solid background rectangle" without
// needing pixel-perfect golden images.
func distinctColorCount(img image.Image) int {
	seen := make(map[color.RGBA64]struct{})
	b := img.Bounds()
	const step = 4
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			r, g, bl, a := img.At(x, y).RGBA()
			seen[color.RGBA64{R: uint16(r), G: uint16(g), B: uint16(bl), A: uint16(a)}] = struct{}{}
		}
	}
	return len(seen)
}

func TestGUIRendersRealContent(t *testing.T) {
	requireE2ETools(t)

	bin := buildSimplenvim(t)
	display := startXvfb(t)

	dir := t.TempDir()
	file := filepath.Join(dir, "e2e.txt")
	if err := os.WriteFile(file, []byte("hello from the e2e test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd := exec.Command(bin, file)
	cmd.Env = append(os.Environ(), "DISPLAY="+display)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start simplenvim: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	windowID := waitForWindow(t, display, 10*time.Second)

	// Poll until the window actually contains a rendered UI. The window
	// existing does not mean Nvim has attached and flushed a frame yet, and
	// under parallel test load that can take much longer than any fixed
	// sleep would allow for.
	var img image.Image
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
		img = screenshotWindow(t, display, windowID)
		if distinctColorCount(img) >= 5 {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	bounds := img.Bounds()
	if bounds.Dx() < 100 || bounds.Dy() < 100 {
		t.Fatalf("screenshot dimensions = %v, want a real-sized window", bounds)
	}

	// A blank/failed render would be a single solid color (or very close to
	// it); real text + line-number gutter + statusline should produce
	// noticeably more than a handful of distinct colors even at a coarse
	// sampling stride.
	if n := distinctColorCount(img); n < 5 {
		t.Fatalf("screenshot has only %d distinct sampled colors, want a real rendered UI (text/gutter/statusline)", n)
	}
}

func TestGUIProcessSpawnsAndCleansUpNvimChild(t *testing.T) {
	requireE2ETools(t)

	bin := buildSimplenvim(t)
	display := startXvfb(t)

	dir := t.TempDir()
	file := filepath.Join(dir, "e2e_lifecycle.txt")
	if err := os.WriteFile(file, []byte("lifecycle check\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd := exec.Command(bin, file)
	cmd.Env = append(os.Environ(), "DISPLAY="+display)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start simplenvim: %v", err)
	}
	appPID := cmd.Process.Pid

	waitForWindow(t, display, 10*time.Second)

	// Poll rather than sleeping a fixed amount: under parallel test load
	// the child can take noticeably longer to appear, and a fixed sleep
	// turns that into a flaky failure.
	spawned := false
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		if hasChildNvim(t, appPID) {
			spawned = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !spawned {
		t.Fatalf("expected simplenvim (pid %d) to have spawned a child nvim process", appPID)
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill simplenvim: %v", err)
	}
	_ = cmd.Wait()

	// Give the OS a moment to reap the child before checking it's gone.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !hasChildNvim(t, appPID) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("nvim child of pid %d was still running %s after the parent was killed", appPID, 3*time.Second)
}

// hasChildNvim reports whether any process on the system currently has
// parentPID as its parent and "nvim" in its command line, using `ps` (a
// standard Linux tool, and this tier already requires Linux).
func hasChildNvim(t *testing.T, parentPID int) bool {
	t.Helper()
	out, err := exec.Command("ps", "-eo", "pid,ppid,args").Output()
	if err != nil {
		t.Fatalf("ps: %v", err)
	}
	ppidStr := strconv.Itoa(parentPID)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[1] == ppidStr && strings.Contains(fields[2], "nvim") {
			return true
		}
	}
	return false
}
