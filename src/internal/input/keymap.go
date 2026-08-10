// Package input translates Gio input events into the vocabulary Nvim's
// msgpack-rpc API expects: `<...>`-notation key strings for nvim_input, and
// button/action/modifier strings for nvim_input_mouse.
package input

import (
	"strings"
	"unicode/utf8"

	"gioui.org/io/key"
)

// specialNames maps Gio's non-printable key.Name constants to the Nvim
// key-notation name that goes inside "<...>" (see `:h key-notation`).
var specialNames = map[key.Name]string{
	key.NameLeftArrow:      "Left",
	key.NameRightArrow:     "Right",
	key.NameUpArrow:        "Up",
	key.NameDownArrow:      "Down",
	key.NameReturn:         "Enter",
	key.NameEnter:          "Enter",
	key.NameEscape:         "Esc",
	key.NameHome:           "Home",
	key.NameEnd:            "End",
	key.NameDeleteBackward: "BS",
	key.NameDeleteForward:  "Del",
	key.NamePageUp:         "PageUp",
	key.NamePageDown:       "PageDown",
	key.NameTab:            "Tab",
	key.NameSpace:          "Space",
	key.NameF1:             "F1",
	key.NameF2:             "F2",
	key.NameF3:             "F3",
	key.NameF4:             "F4",
	key.NameF5:             "F5",
	key.NameF6:             "F6",
	key.NameF7:             "F7",
	key.NameF8:             "F8",
	key.NameF9:             "F9",
	key.NameF10:            "F10",
	key.NameF11:            "F11",
	key.NameF12:            "F12",
}

// pureModifiers are key.Names that represent a modifier key by itself; a
// press/release of just the modifier produces no editor input.
var pureModifiers = map[key.Name]bool{
	key.NameCtrl:    true,
	key.NameShift:   true,
	key.NameAlt:     true,
	key.NameSuper:   true,
	key.NameCommand: true,
}

// EncodeKey converts a Gio key.Event into Nvim key-notation ("a", "!",
// "<C-a>", "<Esc>", ...), or "" if the event shouldn't be sent to Nvim at
// all (key releases, bare modifier presses, or names we don't recognize).
func EncodeKey(e key.Event) string {
	if e.State != key.Press {
		return ""
	}
	if pureModifiers[e.Name] {
		return ""
	}

	if special, ok := specialNames[e.Name]; ok {
		return wrap(special, e.Modifiers, true)
	}

	// Anything left should be a single printable rune: Gio's key.Name
	// already reflects the shifted glyph produced by the active keyboard
	// layout (see the key.Name doc comment), so no layout logic is needed
	// here beyond undoing its "letters are always uppercase" convention
	// when Shift wasn't actually held.
	r, size := utf8.DecodeRuneInString(string(e.Name))
	if size == 0 || size != len(e.Name) {
		return ""
	}

	text := string(r)
	if r >= 'A' && r <= 'Z' && !e.Modifiers.Contain(key.ModShift) {
		text = strings.ToLower(text)
	}

	switch text {
	case "<":
		return wrap("lt", e.Modifiers, true)
	case "\\":
		return wrap("Bslash", e.Modifiers, true)
	}

	// Shift is already encoded in the glyph/case for printable keys, so
	// don't also add an "S-" prefix (that would turn "!" into "<S-!>").
	return wrap(text, e.Modifiers&^key.ModShift, false)
}

// wrap adds the modifier prefix and, if there is one (or alwaysBracket is
// set, as for named special keys), surrounds the result in "<...>".
func wrap(name string, mods key.Modifiers, alwaysBracket bool) string {
	prefix := ModifierPrefix(mods)
	if prefix == "" && !alwaysBracket {
		return name
	}
	return "<" + prefix + name + ">"
}

// ModifierPrefix renders the Nvim key-notation modifier prefix, e.g.
// "C-S-". It is also reused by mouse.go, since nvim_input_mouse's modifier
// argument uses the same "C-"/"S-"/"A-"/"D-" vocabulary.
func ModifierPrefix(mods key.Modifiers) string {
	var b strings.Builder
	if mods.Contain(key.ModCtrl) {
		b.WriteString("C-")
	}
	if mods.Contain(key.ModCommand) || mods.Contain(key.ModSuper) {
		b.WriteString("D-")
	}
	if mods.Contain(key.ModShift) {
		b.WriteString("S-")
	}
	if mods.Contain(key.ModAlt) {
		b.WriteString("A-")
	}
	return b.String()
}
