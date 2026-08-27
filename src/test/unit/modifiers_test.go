package unit_test

import (
	"testing"

	"gioui.org/io/key"

	"github.com/kgfly/SimpleNvimEditor/internal/input"
)

// TestModifierTrackerRecoversWindowsModifiers is the regression test for
// "Ctrl- and Alt- chords do nothing on Windows, but Fn keys work".
//
// Gio v0.10.2 builds Event.Modifiers on Windows from
// GetKeyState(VK_CONTROL) & 0x1000. Win32 puts "key is down" in 0x8000 and
// "key is toggled" in 0x0001; 0x1000 is not a defined bit, so the test is
// always false and every event claims no modifiers are held. Unmodified
// keys such as F1-F12 are unaffected, which is exactly the reported
// asymmetry.
//
// The tracker reconstructs the held set from the modifier key events, which
// arrive correctly on every backend.
func TestModifierTrackerRecoversWindowsModifiers(t *testing.T) {
	chords := []struct {
		desc     string
		modName  key.Name
		pressed  key.Name
		wantKeys string
	}{
		{"ctrl-w is the window prefix", key.NameCtrl, "W", "<C-w>"},
		{"alt-x", key.NameAlt, "X", "<A-x>"},
	}

	for _, c := range chords {
		t.Run(c.desc, func(t *testing.T) {
			var tr input.ModifierTracker

			// Windows reports the modifier keydown itself with an
			// empty modifier set, just like everything else.
			tr.Key(key.Event{Name: c.modName, State: key.Press})

			ev := tr.Key(key.Event{Name: c.pressed, State: key.Press})
			if got := input.EncodeKey(ev); got != c.wantKeys {
				t.Errorf("EncodeKey = %q, want %q", got, c.wantKeys)
			}

			// Releasing the modifier must take effect immediately.
			tr.Key(key.Event{Name: c.modName, State: key.Release})
			if held := tr.Held(); held != 0 {
				t.Errorf("after release, held = %v, want none", held)
			}
			plain := tr.Key(key.Event{Name: c.pressed, State: key.Press})
			if got, want := input.EncodeKey(plain), "x"; c.pressed == "X" && got != want {
				t.Errorf("after release, EncodeKey = %q, want %q", got, want)
			}
		})
	}
}

// TestModifierTrackerCombinesChords covers <C-A-...>, where losing either
// modifier silently changes which mapping fires.
func TestModifierTrackerCombinesChords(t *testing.T) {
	var tr input.ModifierTracker
	tr.Key(key.Event{Name: key.NameCtrl, State: key.Press})
	tr.Key(key.Event{Name: key.NameAlt, State: key.Press})

	ev := tr.Key(key.Event{Name: "D", State: key.Press})
	if got, want := input.EncodeKey(ev), "<C-A-d>"; got != want {
		t.Errorf("EncodeKey = %q, want %q", got, want)
	}

	tr.Key(key.Event{Name: key.NameAlt, State: key.Release})
	ev = tr.Key(key.Event{Name: "D", State: key.Press})
	if got, want := input.EncodeKey(ev), "<C-d>"; got != want {
		t.Errorf("after releasing Alt, EncodeKey = %q, want %q", got, want)
	}
}

// TestModifierTrackerPreservesPlatformModifiers guards against the tracker
// regressing backends that already report modifiers correctly (macOS, X11,
// Wayland): the tracked set is merged in, never substituted for what
// arrived.
func TestModifierTrackerPreservesPlatformModifiers(t *testing.T) {
	var tr input.ModifierTracker
	ev := tr.Key(key.Event{Name: "W", Modifiers: key.ModCtrl, State: key.Press})
	if got, want := input.EncodeKey(ev), "<C-w>"; got != want {
		t.Errorf("EncodeKey = %q, want %q", got, want)
	}
}

// TestModifierTrackerResetClearsStuckModifiers covers the Alt-Tab tail: the
// Alt release is delivered to whichever window gains focus, so without a
// reset the next plain keypress would still look like a chord.
func TestModifierTrackerResetClearsStuckModifiers(t *testing.T) {
	var tr input.ModifierTracker
	tr.Key(key.Event{Name: key.NameAlt, State: key.Press})
	tr.Reset()

	ev := tr.Key(key.Event{Name: "J", State: key.Press})
	if got, want := input.EncodeKey(ev), "j"; got != want {
		t.Errorf("EncodeKey = %q, want %q; a dropped release wedged the modifier", got, want)
	}
}

// TestModifierTrackerModifiersForPointer covers mouse events, which Gio
// builds from the same broken Windows helper and so lose their modifiers
// too (breaking ctrl-click and alt-drag).
func TestModifierTrackerModifiersForPointer(t *testing.T) {
	var tr input.ModifierTracker
	tr.Key(key.Event{Name: key.NameCtrl, State: key.Press})

	if got, want := input.ModifierPrefix(tr.Modifiers(0)), "C-"; got != want {
		t.Errorf("ModifierPrefix = %q, want %q", got, want)
	}
}

// TestBareModifiersProduceNoInput makes sure reconstructing modifiers did
// not start injecting the modifier keys themselves into the buffer.
func TestBareModifiersProduceNoInput(t *testing.T) {
	var tr input.ModifierTracker
	for _, n := range []key.Name{key.NameCtrl, key.NameShift, key.NameAlt, key.NameSuper, key.NameCommand} {
		ev := tr.Key(key.Event{Name: n, State: key.Press})
		if got := input.EncodeKey(ev); got != "" {
			t.Errorf("EncodeKey(bare %v) = %q, want empty", n, got)
		}
		if input.IsTextKey(ev) {
			t.Errorf("bare %v must not be treated as text", n)
		}
		tr.Key(key.Event{Name: n, State: key.Release})
	}
}
