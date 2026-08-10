package integration_test

import (
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/kgfly/SimpleNvimEditor/internal/input"
	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
)

// send encodes a Gio key event exactly as the GUI does and forwards it to
// Nvim, so these tests exercise the real encoder rather than hand-written
// key notation.
func send(t *testing.T, p *nvimproc.Process, ev key.Event) {
	t.Helper()
	encoded := input.EncodeKey(ev)
	if encoded == "" {
		t.Fatalf("EncodeKey(%+v) returned empty; the keystroke would be dropped", ev)
	}
	p.Input(encoded)
}

func startNvim(t *testing.T) *nvimproc.Process {
	t.Helper()
	cmd := requireNvim(t)

	p, err := nvimproc.Spawn(cmd, []string{"-u", "NONE", "-n"}, nil, 80, 24)
	if err != nil {
		t.Fatalf("spawn nvim: %v", err)
	}
	t.Cleanup(func() {
		if p.Nvim != nil {
			_ = p.Nvim.Close()
		}
	})
	return p
}

// waitFor polls until cond returns true, so tests tolerate the inherent
// asynchrony of talking to another process without sleeping for a fixed,
// flaky duration.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestColonEntersCommandMode is the regression test for ":" doing nothing
// in normal mode, which left the user unable to run any Ex command.
func TestColonEntersCommandMode(t *testing.T) {
	p := startNvim(t)

	// ":" arrives as Shift-";" on a US layout, reported by Gio as the
	// already-shifted glyph ":".
	send(t, p, key.Event{Name: ":", Modifiers: key.ModShift, State: key.Press})

	waitFor(t, `nvim to enter command-line mode`, func() bool {
		mode, err := p.Nvim.Mode()
		return err == nil && mode.Mode == "c"
	})
}

// TestColonCommandRuns proves the whole path end to end: ":" opens the
// command line, the command is typed, and <CR> executes it.
func TestColonCommandRuns(t *testing.T) {
	p := startNvim(t)

	send(t, p, key.Event{Name: ":", Modifiers: key.ModShift, State: key.Press})
	for _, r := range "let g:simplenvim_ok = 42" {
		send(t, p, key.Event{Name: key.Name(string(r)), State: key.Press})
	}
	send(t, p, key.Event{Name: key.NameReturn, State: key.Press})

	waitFor(t, "the :let command to take effect", func() bool {
		var got int
		if err := p.Nvim.Var("simplenvim_ok", &got); err != nil {
			return false
		}
		return got == 42
	})
}

// TestCtrlWSplitsWindows is the regression test for "<C-w>s" / "<C-w>v"
// doing nothing, which left the user unable to split panes.
func TestCtrlWSplitsWindows(t *testing.T) {
	cases := []struct {
		desc string
		key  key.Event
	}{
		{"horizontal split via <C-w>s", key.Event{Name: "S", State: key.Press}},
		{"vertical split via <C-w>v", key.Event{Name: "V", State: key.Press}},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			p := startNvim(t)

			before, err := p.Nvim.Windows()
			if err != nil {
				t.Fatalf("list windows: %v", err)
			}
			if len(before) != 1 {
				t.Fatalf("expected to start with 1 window, got %d", len(before))
			}

			send(t, p, key.Event{Name: "W", Modifiers: key.ModCtrl, State: key.Press})
			send(t, p, tc.key)

			waitFor(t, "the window to split", func() bool {
				wins, err := p.Nvim.Windows()
				return err == nil && len(wins) == 2
			})
		})
	}
}
