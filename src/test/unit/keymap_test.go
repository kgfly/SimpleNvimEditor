package unit_test

import (
	"testing"

	"gioui.org/io/key"

	"github.com/kgfly/SimpleNvimEditor/internal/input"
)

func TestEncodeKeyIgnoresReleaseAndBareModifiers(t *testing.T) {
	cases := []key.Event{
		{Name: "A", State: key.Release},
		{Name: key.NameCtrl, State: key.Press},
		{Name: key.NameShift, State: key.Press},
		{Name: key.NameAlt, State: key.Press},
		{Name: key.NameSuper, State: key.Press},
		{Name: key.NameCommand, State: key.Press},
	}
	for _, e := range cases {
		if got := input.EncodeKey(e); got != "" {
			t.Errorf("EncodeKey(%+v) = %q, want \"\"", e, got)
		}
	}
}

func TestEncodeKeyPrintableRunes(t *testing.T) {
	cases := []struct {
		name string
		e    key.Event
		want string
	}{
		{"lowercase letter, no shift", key.Event{Name: "A", State: key.Press}, "a"},
		{"uppercase letter, shift held", key.Event{Name: "A", Modifiers: key.ModShift, State: key.Press}, "A"},
		{"digit passthrough", key.Event{Name: "1", State: key.Press}, "1"},
		{"already-shifted symbol", key.Event{Name: "!", State: key.Press}, "!"},
		{"colon", key.Event{Name: ":", State: key.Press}, ":"},
		{"less-than escaped", key.Event{Name: "<", State: key.Press}, "<lt>"},
		{"backslash escaped", key.Event{Name: "\\", State: key.Press}, "<Bslash>"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := input.EncodeKey(c.e); got != c.want {
				t.Fatalf("EncodeKey(%+v) = %q, want %q", c.e, got, c.want)
			}
		})
	}
}

func TestEncodeKeySpecialNamesAlwaysBracketed(t *testing.T) {
	cases := []struct {
		e    key.Event
		want string
	}{
		{key.Event{Name: key.NameEscape, State: key.Press}, "<Esc>"},
		{key.Event{Name: key.NameReturn, State: key.Press}, "<Enter>"},
		{key.Event{Name: key.NameEnter, State: key.Press}, "<Enter>"},
		{key.Event{Name: key.NameSpace, State: key.Press}, "<Space>"},
		{key.Event{Name: key.NameTab, State: key.Press}, "<Tab>"},
		{key.Event{Name: key.NameLeftArrow, State: key.Press}, "<Left>"},
		{key.Event{Name: key.NameDeleteBackward, State: key.Press}, "<BS>"},
		{key.Event{Name: key.NameDeleteForward, State: key.Press}, "<Del>"},
		{key.Event{Name: key.NameF5, State: key.Press}, "<F5>"},
		{key.Event{Name: key.NameTab, Modifiers: key.ModShift, State: key.Press}, "<S-Tab>"},
		{key.Event{Name: key.NameLeftArrow, Modifiers: key.ModShift, State: key.Press}, "<S-Left>"},
	}
	for _, c := range cases {
		if got := input.EncodeKey(c.e); got != c.want {
			t.Errorf("EncodeKey(%+v) = %q, want %q", c.e, got, c.want)
		}
	}
}

func TestEncodeKeyModifierCombinations(t *testing.T) {
	cases := []struct {
		name string
		e    key.Event
		want string
	}{
		{"ctrl+letter", key.Event{Name: "A", Modifiers: key.ModCtrl, State: key.Press}, "<C-a>"},
		{"ctrl+digit", key.Event{Name: "1", Modifiers: key.ModCtrl, State: key.Press}, "<C-1>"},
		{"command (mac cmd) + letter", key.Event{Name: "A", Modifiers: key.ModCommand, State: key.Press}, "<D-a>"},
		{"super (win/linux logo) + letter", key.Event{Name: "A", Modifiers: key.ModSuper, State: key.Press}, "<D-a>"},
		{"alt + letter", key.Event{Name: "A", Modifiers: key.ModAlt, State: key.Press}, "<A-a>"},
		// Ctrl collapses a letter to a control byte, discarding case, so a
		// Ctrl+Shift chord needs an explicit "S-" to stay distinct from <C-a>.
		{"ctrl+shift+letter", key.Event{Name: "A", Modifiers: key.ModCtrl | key.ModShift, State: key.Press}, "<C-S-a>"},
		{"all four modifiers", key.Event{Name: "A", Modifiers: key.ModCtrl | key.ModCommand | key.ModShift | key.ModAlt, State: key.Press}, "<C-D-S-A-a>"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := input.EncodeKey(c.e); got != c.want {
				t.Fatalf("EncodeKey(%+v) = %q, want %q", c.e, got, c.want)
			}
		})
	}
}

func TestEncodeKeyUnrecognizedMultiRuneNameIsIgnored(t *testing.T) {
	// A key.Name that isn't in specialNames and isn't a single rune (e.g. an
	// unmapped platform-specific name) should be safely dropped, not panic
	// or send garbage to Nvim.
	e := key.Event{Name: "SomeUnknownKey", State: key.Press}
	if got := input.EncodeKey(e); got != "" {
		t.Fatalf("EncodeKey(%+v) = %q, want \"\"", e, got)
	}
}

func TestModifierPrefixOrderingAndContents(t *testing.T) {
	cases := []struct {
		mods key.Modifiers
		want string
	}{
		{0, ""},
		{key.ModCtrl, "C-"},
		{key.ModShift, "S-"},
		{key.ModAlt, "A-"},
		{key.ModCommand, "D-"},
		{key.ModSuper, "D-"},
		{key.ModCtrl | key.ModShift, "C-S-"},
		{key.ModCtrl | key.ModCommand | key.ModShift | key.ModAlt, "C-D-S-A-"},
	}
	for _, c := range cases {
		if got := input.ModifierPrefix(c.mods); got != c.want {
			t.Errorf("ModifierPrefix(%v) = %q, want %q", c.mods, got, c.want)
		}
	}
}
