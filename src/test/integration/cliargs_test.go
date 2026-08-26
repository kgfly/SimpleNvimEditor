package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kgfly/SimpleNvimEditor/internal/cli"
	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// spawnWithArgs starts a real Nvim with userArgs exactly as the App would,
// i.e. whatever cli.Parse produced for the command line. Config-level
// extraArgs stay at "-u NONE -n" so the host's personal init.lua can't
// change the outcome.
func spawnWithArgs(t *testing.T, userArgs []string) *nvimproc.Process {
	t.Helper()
	nvimPath := requireNvim(t)

	proc, err := nvimproc.Spawn(nvimPath, []string{"-u", "NONE", "-n"}, userArgs, 60, 20)
	if err != nil {
		t.Fatalf("nvimproc.Spawn(%q): %v", userArgs, err)
	}
	t.Cleanup(func() {
		// Buffers opened by a pass-through "-c edit" may be modified,
		// which would leave `confirm qa` waiting on a prompt; drop the
		// modified flag first so cleanup can't hang the suite.
		_ = proc.Nvim.Command("silent! bufdo set nomodified")
		proc.RequestQuit()
		select {
		case <-proc.Exited:
			return
		case <-time.After(2 * time.Second):
		}
		t.Logf("nvim did not exit gracefully after RequestQuit; force-closing")
		_ = proc.Nvim.Close()
		<-proc.Exited
	})
	return proc
}

// eval returns the trimmed output of `echo expr` from the live Nvim, which
// is ground truth about what the arguments actually did — independent of
// our own redraw-derived mirror.
func eval(t *testing.T, proc *nvimproc.Process, expr string) string {
	t.Helper()
	out, err := proc.Nvim.CommandOutput("echo " + expr)
	if err != nil {
		t.Fatalf("echo %s: %v", expr, err)
	}
	return strings.TrimSpace(out)
}

// TestLauncherArgsReachNvim is the end-of-the-line check for the launcher
// invocation
//
//	simplenvim --maximized -- -c term -c edit <file>
//
// It runs the real argument parser and feeds its output to a real Nvim, so
// it fails if either half of the chain (parsing or forwarding) regresses.
func TestLauncherArgsReachNvim(t *testing.T) {
	dir := t.TempDir()
	todo := filepath.Join(dir, "n.todo")
	if err := os.WriteFile(todo, []byte("write the tests\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	opts, err := cli.Parse("simplenvim", []string{"--maximized", "--", "-c", "term", "-c", "edit", todo}, os.Stderr)
	if err != nil {
		t.Fatalf("cli.Parse: %v", err)
	}
	if !opts.Maximized {
		t.Fatal("Maximized = false, want true")
	}

	proc := spawnWithArgs(t, opts.NvimArgs)
	s := uistate.New()
	drainUntilFlush(t, proc, s)

	// "-c term" must have produced a terminal buffer...
	if got := eval(t, proc, `len(filter(map(range(1, bufnr('$')), 'getbufvar(v:val, "&buftype")'), 'v:val ==# "terminal"'))`); got == "0" {
		t.Error("no terminal buffer exists, so \"-c term\" did not reach nvim")
	}

	// ...and the file must have been loaded as buffer 1.
	//
	// Note what nvim actually does with this command line: "-c" takes
	// exactly *one* argument, so "-c edit <file>" is "-c edit" plus a
	// positional file argument — not "edit <file>" as one command. The
	// file therefore becomes buffer 1 (nvim's usual treatment of a file
	// argument) while the bare ":edit" merely reloads the current
	// buffer, which by then is the terminal "-c term" just opened. So
	// the terminal, not the file, is what stays on screen.
	//
	// This is nvim's own argument grammar, identical to running the same
	// line against plain `nvim`, and it's precisely the behaviour a
	// pass-through must preserve: our job is to forward arguments
	// verbatim, not to second-guess them.
	if got := eval(t, proc, "bufname(1)"); !strings.HasSuffix(got, "n.todo") {
		t.Errorf("buffer 1 = %q, want the n.todo file argument", got)
	}
	if got := eval(t, proc, "getbufline(1, 1)[0]"); got != "write the tests" {
		t.Errorf("buffer 1 line 1 = %q, want %q", got, "write the tests")
	}
	if got := eval(t, proc, "&buftype"); got != "terminal" {
		t.Errorf("current buftype = %q, want %q (the -c term buffer stays current)", got, "terminal")
	}
}

// TestPassThroughArgsPrecedeFiles pins the ordering cli.Parse promises:
// nvim flags are positional-hungry, so "-O" has to arrive before the files
// it applies to or it opens one window instead of two vertical splits.
func TestPassThroughArgsPrecedeFiles(t *testing.T) {
	dir := t.TempDir()
	var files []string
	for _, name := range []string{"left.txt", "right.txt"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name+"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		files = append(files, path)
	}

	argv := append(append([]string{}, files...), "--", "-O")
	opts, err := cli.Parse("simplenvim", argv, os.Stderr)
	if err != nil {
		t.Fatalf("cli.Parse: %v", err)
	}

	proc := spawnWithArgs(t, opts.NvimArgs)
	s := uistate.New()
	drainUntilFlush(t, proc, s)

	if got := eval(t, proc, "winnr('$')"); got != "2" {
		t.Errorf("window count = %q, want \"2\" (-O should split both files vertically)", got)
	}
}

// TestNoUserArgsStillStarts guards the plain "open an empty editor" case
// against an off-by-one in argument assembly.
func TestNoUserArgsStillStarts(t *testing.T) {
	opts, err := cli.Parse("simplenvim", nil, os.Stderr)
	if err != nil {
		t.Fatalf("cli.Parse: %v", err)
	}

	proc := spawnWithArgs(t, opts.NvimArgs)
	s := uistate.New()
	drainUntilFlush(t, proc, s)

	if got := eval(t, proc, "expand('%')"); got != "" {
		t.Errorf("current buffer name = %q, want empty", got)
	}
}
