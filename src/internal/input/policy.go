package input

import "gioui.org/io/key"

// Policy captures the one input decision that is a genuine matter of taste
// rather than a fact about the platform: what the Alt/Option key means.
type Policy struct {
	// AltIsMeta routes Alt-chords to Nvim as <A-...> mappings instead of
	// letting the platform turn them into text.
	//
	// This only has teeth on macOS, where Option is a composing modifier:
	// Option-a inserts "å" and Option-Shift-a inserts "Å" via
	// NSTextInputClient, so with AltIsMeta false an <A-a> mapping can
	// never fire. That is the reported macOS Alt-Shift bug.
	//
	// On every other platform Alt is already a pure command modifier and
	// no text is generated. That is verifiable in Gio v0.10.2 rather than
	// folklore:
	//
	//   - Windows: Alt-key raises WM_SYSKEYDOWN/WM_SYSCHAR, and
	//     app/os_windows.go handles only WM_CHAR/WM_UNICHAR, so no
	//     EditEvent is ever emitted.
	//   - X11/Wayland: app/internal/xkb/xkb_unix.go refuses to produce
	//     text whenever Ctrl, Alt or Super is active.
	//
	// So false is not merely a preference there, it is wrong: the key
	// path would defer to an EditEvent that never arrives and the
	// keystroke would vanish. Defaulting to true everywhere also means a
	// config shared between machines behaves identically on all three.
	AltIsMeta bool
}

// DefaultPolicy returns the policy used when the user has expressed no
// preference.
func DefaultPolicy() Policy {
	return Policy{AltIsMeta: true}
}

// MetaHeld reports whether mods contain an Alt that this policy treats as
// Meta rather than as a text-composing modifier.
//
// It is asked from two directions, which is why it is a method rather than
// an inline test: the key path uses it to claim Alt-chords (IsTextKey), and
// the text path uses it to disown the text those chords generate anyway
// (see App.onEdit). Both must agree, or the keystroke is either duplicated
// or lost.
func (p Policy) MetaHeld(mods key.Modifiers) bool {
	return p.AltIsMeta && mods.Contain(key.ModAlt)
}

// IsTextKey reports whether a key.Event describes a keystroke that the
// platform ALSO delivers as a key.EditEvent, and which must therefore be
// ignored by the key path to avoid inserting the character twice.
//
// Both macOS and X11 route printable characters through the text-input
// layer (NSTextInputClient's insertText: / xkb's compose machinery) in
// addition to reporting the raw key, because that layer is what implements
// dead keys, IMEs and dictation. Observed on macOS 26 with Gio v0.10.2:
//
//	'a'       -> EditEvent("a") + key.Event("A")
//	Shift+'a' -> EditEvent("A") + key.Event("A", Shift)
//	Space     -> EditEvent(" ") + key.Event(Space)
//	Option+x  -> EditEvent("≈") + key.Event("≈", Alt)
//	Esc/CR/BS/Tab/arrows -> key.Event only
//	Ctrl+w, Cmd+v        -> key.Event only
//
// The rule: a single printable rune that EncodeKey does not already
// recognise by name, with no command modifier held. Ctrl, Command and Super
// always suppress text generation (they produce commands like "<C-w>");
// Shift merely selects a different glyph and so still counts as text. Alt
// is the judgement call, and belongs to Policy.
//
// This cannot be derived from EncodeKey's output: it also brackets "<lt>",
// "<Bslash>", "<Space>" and "<A-≈>", every one of which is real text.
func (p Policy) IsTextKey(e key.Event) bool {
	if e.State != key.Press {
		return false
	}
	if e.Modifiers&(key.ModCtrl|key.ModCommand|key.ModSuper) != 0 {
		return false
	}
	if p.MetaHeld(e.Modifiers) {
		return false
	}
	// Space is a named key but is genuinely text: it arrives as
	// EditEvent(" ") too, so the text path must own it.
	if e.Name == key.NameSpace {
		return true
	}
	if namedKeys[e.Name] || IsModifierKey(e.Name) {
		return false
	}
	return isSingleRune(e.Name)
}
