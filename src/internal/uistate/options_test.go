package uistate

import "testing"

func TestOptionSetStoresGuiFont(t *testing.T) {
	state := New()
	state.Apply([][]interface{}{
		{"option_set", []interface{}{"guifont", "Hack Nerd Font Mono:h17:b"}},
	})

	if got, want := state.GuiFont(), "Hack Nerd Font Mono:h17:b"; got != want {
		t.Fatalf("GuiFont = %q, want %q", got, want)
	}
}

func TestOptionSetIgnoresOtherOptionsAndMalformedRows(t *testing.T) {
	state := New()
	state.Apply([][]interface{}{
		{"option_set", []interface{}{"guifont", "monospace:h14"}},
		{"option_set", []interface{}{"linespace", 2}, "invalid"},
	})

	if got, want := state.GuiFont(), "monospace:h14"; got != want {
		t.Fatalf("GuiFont = %q, want %q", got, want)
	}
}
