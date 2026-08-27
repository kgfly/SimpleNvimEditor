package unit_test

import (
	"testing"

	"gioui.org/io/key"

	"github.com/kgfly/SimpleNvimEditor/internal/input"
)

func TestIsTextKeyPressOnly(t *testing.T) {
	e := key.Event{Name: "A", State: key.Release}
	if input.IsTextKey(e) {
		t.Fatal("release events should not be text keys")
	}
}

func TestIsTextKeyCtrlSuppresses(t *testing.T) {
	for _, mod := range []key.Modifiers{key.ModCtrl, key.ModCommand, key.ModSuper} {
		e := key.Event{Name: "A", State: key.Press, Modifiers: mod}
		if input.IsTextKey(e) {
			t.Fatalf("modifier %v should suppress text key", mod)
		}
	}
}

func TestIsTextKeySpaceIsText(t *testing.T) {
	e := key.Event{Name: key.NameSpace, State: key.Press}
	if !input.IsTextKey(e) {
		t.Fatal("Space should be a text key")
	}
}

func TestIsTextKeyNamedKeysNotText(t *testing.T) {
	names := []key.Name{
		key.NameEscape, key.NameReturn, key.NameTab,
		key.NameLeftArrow, key.NameHome, key.NameF1,
		key.NameDeleteBackward, key.NameDeleteForward,
	}
	for _, n := range names {
		e := key.Event{Name: n, State: key.Press}
		if input.IsTextKey(e) {
			t.Fatalf("%v should not be a text key", n)
		}
	}
}

func TestIsTextKeyPrintableRune(t *testing.T) {
	cases := []struct {
		name key.Name
		want bool
	}{
		{"A", true},
		{"1", true},
		{"≈", true},
		{"!", true},
	}
	for _, c := range cases {
		e := key.Event{Name: c.name, State: key.Press}
		if got := input.IsTextKey(e); got != c.want {
			t.Errorf("IsTextKey(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestIsTextKeyShiftIsText(t *testing.T) {
	e := key.Event{Name: "A", State: key.Press, Modifiers: key.ModShift}
	if !input.IsTextKey(e) {
		t.Fatal("Shift+letter should be a text key")
	}
}

// TestIsTextKeyAltFollowsPolicy pins down the macOS Alt-Shift bug. By
// default Alt is Meta, so an Alt-chord must NOT be claimed by the text
// path — otherwise the key path skips it and the <A-...> mapping never
// fires (macOS), or the keystroke vanishes entirely because no EditEvent
// is ever sent (Windows, X11, Wayland).
func TestIsTextKeyAltFollowsPolicy(t *testing.T) {
	alt := key.Event{Name: "≈", State: key.Press, Modifiers: key.ModAlt}
	altShift := key.Event{Name: "A", State: key.Press, Modifiers: key.ModAlt | key.ModShift}

	meta := input.Policy{AltIsMeta: true}
	for _, e := range []key.Event{alt, altShift} {
		if meta.IsTextKey(e) {
			t.Errorf("AltIsMeta: %+v must reach the key path, not the text path", e)
		}
	}

	compose := input.Policy{AltIsMeta: false}
	for _, e := range []key.Event{alt, altShift} {
		if !compose.IsTextKey(e) {
			t.Errorf("AltIsMeta=false: %+v should compose text", e)
		}
	}

	if !input.DefaultPolicy().AltIsMeta {
		t.Error("Alt should be Meta by default")
	}
}

func TestIsTextKeyEmptyName(t *testing.T) {
	e := key.Event{Name: "", State: key.Press}
	if input.IsTextKey(e) {
		t.Fatal("empty name should not be a text key")
	}
}

func TestEncodeText(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"hello", "hello"},
		{"<", "<lt>"},
		{"a<b", "a<lt>b"},
		{"<<", "<lt><lt>"},
		{"no special", "no special"},
		{"", ""},
	}
	for _, c := range cases {
		if got := input.EncodeText(c.in); got != c.want {
			t.Errorf("EncodeText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBackspaces(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, ""},
		{-1, ""},
		{1, "<BS>"},
		{3, "<BS><BS><BS>"},
	}
	for _, c := range cases {
		if got := input.Backspaces(c.n); got != c.want {
			t.Errorf("Backspaces(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
