package integration_test

import (
	"strings"
	"testing"

	"gioui.org/io/key"

	"github.com/kgfly/SimpleNvimEditor/internal/input"
	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
)

// sendVia encodes through a ModifierTracker, mirroring what App.onKey and
// App.onEdit do together, and forwards the result to Nvim.
//
// Modelling both paths matters: a printable key with no command modifier is
// claimed by the text path, so a harness that only ran the key path would
// silently drop ordinary typing and report failures that the real app does
// not have.
func sendVia(t *testing.T, p *nvimproc.Process, tr *input.ModifierTracker, ev key.Event) {
	t.Helper()
	e := tr.Key(ev)
	if input.DefaultPolicy().IsTextKey(e) {
		if e.State == key.Press {
			// Stand in for the platform's EditEvent.
			p.Input(input.EncodeText(textFor(e)))
		}
		return
	}
	if encoded := input.EncodeKey(e); encoded != "" {
		p.Input(encoded)
	}
}

// textFor is the character the platform's text layer would produce for a
// printable key event: Gio uppercases letter names, so unshifted ones are
// folded back down.
func textFor(e key.Event) string {
	if e.Name == key.NameSpace {
		return " "
	}
	s := string(e.Name)
	if !e.Modifiers.Contain(key.ModShift) {
		return strings.ToLower(s)
	}
	return s
}

// windowsStream replays a chord the way Gio's Windows backend actually
// reports it: the modifier keydown arrives, and then every subsequent event
// claims no modifiers are held, because getModifiers() tests GetKeyState
// against the undefined bit 0x1000 and so always returns zero.
func windowsStream(modifier key.Name, pressed key.Name) []key.Event {
	return []key.Event{
		{Name: modifier, State: key.Press},
		{Name: pressed, State: key.Press},
		{Name: pressed, State: key.Release},
		{Name: modifier, State: key.Release},
	}
}

// TestWindowsStyleCtrlChordFiresMapping is the end-to-end regression test
// for "ctrl-key mappings do not work on Windows". It defines a real mapping
// in a real Nvim and proves the modifier-stripped event stream still fires
// it.
func TestWindowsStyleCtrlChordFiresMapping(t *testing.T) {
	cases := []struct {
		desc     string
		mapping  string
		modifier key.Name
		pressed  key.Name
		variable string
	}{
		{"ctrl chord", "<C-k>", key.NameCtrl, "K", "got_ctrl"},
		{"alt chord", "<A-j>", key.NameAlt, "J", "got_alt"},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			p := startNvim(t)

			// A normal-mode mapping is the most faithful test: it
			// only fires if Nvim receives the exact chord.
			if err := p.Nvim.Command(
				"nnoremap " + tc.mapping + " :let g:" + tc.variable + " = 1<CR>",
			); err != nil {
				t.Fatalf("define mapping: %v", err)
			}

			var tr input.ModifierTracker
			for _, ev := range windowsStream(tc.modifier, tc.pressed) {
				sendVia(t, p, &tr, ev)
			}

			waitFor(t, "the "+tc.desc+" mapping to fire", func() bool {
				var got int
				if err := p.Nvim.Var(tc.variable, &got); err != nil {
					return false
				}
				return got == 1
			})
		})
	}
}

// TestWindowsStyleReleaseDoesNotStick proves the tracker un-sticks the
// modifier, so ordinary typing still works after a chord. Without this, a
// stuck Ctrl would turn every later keystroke into a command.
//
// The chord is deliberately <C-k>, not <C-w>: <C-w> is a prefix and leaves
// Nvim waiting for a second key, which would swallow the ":" and make this
// test fail for a reason that has nothing to do with modifier tracking.
func TestWindowsStyleReleaseDoesNotStick(t *testing.T) {
	p := startNvim(t)

	if err := p.Nvim.Command("nnoremap <C-k> <Nop>"); err != nil {
		t.Fatalf("define mapping: %v", err)
	}

	var tr input.ModifierTracker
	for _, ev := range windowsStream(key.NameCtrl, "K") {
		sendVia(t, p, &tr, ev)
	}

	// Now type a plain ":" — it must reach the command line
	// rather than being swallowed as a chord.
	sendVia(t, p, &tr, key.Event{Name: ":", Modifiers: key.ModShift, State: key.Press})

	waitFor(t, "nvim to enter command-line mode after the chord", func() bool {
		mode, err := p.Nvim.Mode()
		return err == nil && mode.Mode == "c"
	})
}

// TestAltShiftChordFiresMapping is the end-to-end regression test for "on
// mac alt-shift-key mapping does not work". Nvim spells Alt-Shift-a as
// "<A-A>" (keytrans confirms it is identical to "<A-S-a>"), and with Alt
// treated as Meta the chord reaches the key path instead of being consumed
// by macOS's Option-composition.
func TestAltShiftChordFiresMapping(t *testing.T) {
	p := startNvim(t)

	if err := p.Nvim.Command("nnoremap <A-A> :let g:got_alt_shift = 1<CR>"); err != nil {
		t.Fatalf("define mapping: %v", err)
	}

	var tr input.ModifierTracker
	events := []key.Event{
		{Name: key.NameAlt, State: key.Press},
		{Name: key.NameShift, State: key.Press},
		// macOS reports the unshifted-layout glyph, uppercased.
		{Name: "A", State: key.Press},
		{Name: "A", State: key.Release},
		{Name: key.NameShift, State: key.Release},
		{Name: key.NameAlt, State: key.Release},
	}
	for _, ev := range events {
		sendVia(t, p, &tr, ev)
	}

	waitFor(t, "the alt-shift mapping to fire", func() bool {
		var got int
		if err := p.Nvim.Var("got_alt_shift", &got); err != nil {
			return false
		}
		return got == 1
	})
}

// TestAltShiftIsDistinctFromAlt guards the actual complaint: Alt-Shift-a
// must not be delivered as plain Alt-a. If the Shift were lost, the <A-a>
// mapping would fire and this test would catch it.
func TestAltShiftIsDistinctFromAlt(t *testing.T) {
	p := startNvim(t)

	if err := p.Nvim.Command("nnoremap <A-a> :let g:fired = 'lower'<CR>"); err != nil {
		t.Fatalf("define mapping: %v", err)
	}
	if err := p.Nvim.Command("nnoremap <A-A> :let g:fired = 'upper'<CR>"); err != nil {
		t.Fatalf("define mapping: %v", err)
	}

	var tr input.ModifierTracker
	for _, ev := range []key.Event{
		{Name: key.NameAlt, State: key.Press},
		{Name: key.NameShift, State: key.Press},
		{Name: "A", State: key.Press},
	} {
		sendVia(t, p, &tr, ev)
	}

	waitFor(t, "the uppercase alt mapping to fire", func() bool {
		var got string
		if err := p.Nvim.Var("fired", &got); err != nil {
			return false
		}
		if got == "lower" {
			t.Fatal("Alt-Shift-a fired the <A-a> mapping; the Shift was lost")
		}
		return got == "upper"
	})
}
