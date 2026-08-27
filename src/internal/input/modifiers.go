package input

import "gioui.org/io/key"

// modifierBits maps each bare modifier key.Name to the modifier bit holding
// it contributes. It is the single source of truth for "is this key nothing
// but a modifier?", which three separate code paths need to know:
// EncodeKey (a lone modifier is not input), IsTextKey (nor is it text), and
// ModifierTracker (which is interested in exactly these keys).
var modifierBits = map[key.Name]key.Modifiers{
	key.NameCtrl:    key.ModCtrl,
	key.NameShift:   key.ModShift,
	key.NameAlt:     key.ModAlt,
	key.NameSuper:   key.ModSuper,
	key.NameCommand: key.ModCommand,
}

// IsModifierKey reports whether n names a modifier key by itself, as opposed
// to a key that may merely be modified.
func IsModifierKey(n key.Name) bool {
	_, ok := modifierBits[n]
	return ok
}

// ModifierTracker reconstructs which modifier keys are currently held by
// watching the key events themselves, rather than trusting the modifier set
// each event carries.
//
// That distrust is earned. Gio v0.10.2's Windows backend builds
// Event.Modifiers from GetKeyState(VK_CONTROL) & 0x1000 (see
// app/os_windows.go getModifiers). Win32 reports "key is currently down" in
// the high bit, 0x8000, and "key is toggled" in the low bit, 0x0001; 0x1000
// is not a defined GetKeyState bit and is never set. Every key event on
// Windows therefore arrives claiming no modifiers are held, which is exactly
// the reported symptom: Ctrl- and Alt- chords do nothing, while unmodified
// keys such as F1-F12 work perfectly. Mouse events are built from the same
// broken helper, so they lose their modifiers too.
//
// Tracking press/release of the modifier keys is portable and race-free.
// Every backend emits them in the same stream as the keys they modify
// (Windows VK_CONTROL/VK_MENU/VK_SHIFT, macOS flagsChanged, X11 and Wayland
// Control_L/Alt_L via xkb), so the reconstructed set always describes the
// keyboard as it was when the event was generated. Re-reading the live
// keyboard state at handling time would instead describe the keyboard now,
// and would drop modifiers whenever a fast typist releases a chord before
// the frame that processes it.
//
// The tracked set is merged into, never substituted for, what the platform
// reports, so backends that already get this right lose nothing.
type ModifierTracker struct {
	held key.Modifiers
}

// Key records e's effect on the held set and returns e with the tracked
// modifiers merged in. Tracking happens before merging so that releasing a
// modifier takes effect on its own release event.
func (t *ModifierTracker) Key(e key.Event) key.Event {
	if bit, ok := modifierBits[e.Name]; ok {
		switch e.State {
		case key.Press:
			t.held |= bit
		case key.Release:
			t.held &^= bit
		}
	}
	e.Modifiers = t.Modifiers(e.Modifiers)
	return e
}

// Modifiers merges the tracked modifiers into mods. It is for events that
// report a modifier set but can never change it, such as pointer events.
func (t *ModifierTracker) Modifiers(mods key.Modifiers) key.Modifiers {
	return mods | t.held
}

// Held reports the modifiers currently believed to be down.
func (t *ModifierTracker) Held() key.Modifiers {
	return t.held
}

// Reset forgets every held modifier.
//
// This is what keeps a missed release from wedging the editor. Releasing a
// modifier while another window has focus (the tail of every Alt-Tab) sends
// that release somewhere else, so without a reset on focus change the next
// plain keypress would still look like a chord.
func (t *ModifierTracker) Reset() {
	t.held = 0
}
