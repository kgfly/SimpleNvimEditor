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
// gioui.org/io/key. The bare modifier names live in modifierBits instead of
// being repeated here; NameSpace is deliberately absent, see IsTextKey.
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

// IsTextKey reports whether a keystroke is one the platform also delivers
// as a key.EditEvent, using the default policy. See Policy.IsTextKey.
func IsTextKey(e key.Event) bool {
	return DefaultPolicy().IsTextKey(e)
}

// isSingleRune reports whether n consists of exactly one printable rune,
// which is what distinguishes a typed character from a named key.
func isSingleRune(n key.Name) bool {
	r, size := utf8.DecodeRuneInString(string(n))
	if size == 0 || size != len(n) {
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
