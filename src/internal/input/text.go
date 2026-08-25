package input

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"gioui.org/io/key"
)

// namedKeys is every key.Name that Gio uses for a non-text key. Most are
// spelled with a single printable glyph ("←", "⏎", "⌫", "⎋", ...), so they
// cannot be distinguished from a typed character by inspecting the rune —
// they have to be enumerated. Mirrors the key.Name* constants in
// gioui.org/io/key. NameSpace is deliberately absent; see IsTextKey.
var namedKeys = map[key.Name]bool{
	key.NameLeftArrow:      true,
	key.NameRightArrow:     true,
	key.NameUpArrow:        true,
	key.NameDownArrow:      true,
	key.NameReturn:         true,
	key.NameEnter:          true,
	key.NameEscape:         true,
	key.NameHome:           true,
	key.NameEnd:            true,
	key.NameDeleteBackward: true,
	key.NameDeleteForward:  true,
	key.NamePageUp:         true,
	key.NamePageDown:       true,
	key.NameTab:            true,
	key.NameBack:           true,
	key.NameCtrl:           true,
	key.NameShift:          true,
	key.NameAlt:            true,
	key.NameSuper:          true,
	key.NameCommand:        true,
	key.NameF1:             true,
	key.NameF2:             true,
	key.NameF3:             true,
	key.NameF4:             true,
	key.NameF5:             true,
	key.NameF6:             true,
	key.NameF7:             true,
	key.NameF8:             true,
	key.NameF9:             true,
	key.NameF10:            true,
	key.NameF11:            true,
	key.NameF12:            true,
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
// recognise by name, with no Ctrl/Command/Super held. Ctrl and Command
// suppress text generation (they produce commands like "<C-w>"), whereas
// Shift and Alt merely select a different glyph and so still count as text.
//
// This cannot be derived from EncodeKey's output: it also brackets "<lt>",
// "<Bslash>", "<Space>" and "<A-≈>", every one of which is real text.
func IsTextKey(e key.Event) bool {
	if e.State != key.Press {
		return false
	}
	if e.Modifiers.Contain(key.ModCtrl) ||
		e.Modifiers.Contain(key.ModCommand) ||
		e.Modifiers.Contain(key.ModSuper) {
		return false
	}
	//is a named key but is genuinely text: it arrives as
	// EditEvent(" ") too, so the text path must own it.
	if e.Name == key.NameSpace {
		return true
	}
	if namedKeys[e.Name] {
		return false
	}
	r, size := utf8.DecodeRuneInString(string(e.Name))
	if size == 0 || size != len(e.Name) {
		return false
	}
	return unicode.IsPrint(r)
}

// EncodeText prepares literal text (produced by an input method: macOS
// Dictation, a CJK IME, the emoji picker, or an ordinary keypress) for
// nvim_input.
//
// nvim_input reads its argument as if it were the right-hand side of a
// mapping, so "<" opens a key-notation escape ("<Esc>", "<C-w>", ...). A
// literal "<" must therefore be written "<lt>", or Nvim will swallow the
// rest of the phrase looking for a closing ">". Nothing else is special:
// ">" only has meaning after an unescaped "<", and backslashes are literal.
func EncodeText(s string) string {
	if !strings.ContainsRune(s, '<') {
		return s
	}
	return strings.ReplaceAll(s, "<", "<lt>")
}

// Backspaces returns the key-notation string that deletes n characters
// before the cursor, which is how an input method's "replace the text I
// previously marked" instruction is expressed to Nvim.
func Backspaces(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("<BS>", n)
}
