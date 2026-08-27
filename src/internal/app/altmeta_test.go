package editorapp

import (
	"testing"

	"gioui.org/io/key"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
	"github.com/kgfly/SimpleNvimEditor/internal/input"
)

// pressAlt puts the app in the state macOS is in while Option is held.
func pressAlt(a *App) {
	a.mods.Key(key.Event{Name: key.NameAlt, State: key.Press})
}

// TestAltMetaDiscardsComposedText is the regression test for Alt-chords
// being inserted as text *and* dispatched as <A-...> on macOS.
//
// Gio's macOS driver calls interpretKeyEvents from gio_onKeys BEFORE it
// emits the key.Event (app/os_macos.go), so Option-a produces the EditEvent
// for "å" first and the key.Event second. Treating Alt as Meta makes the
// key path claim the chord, so the composed text is a duplicate: without
// this guard the user gets "å" in the buffer as well as the mapping firing.
func TestAltMetaDiscardsComposedText(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	if !a.policy.AltIsMeta {
		t.Fatal("default policy should treat Alt as Meta")
	}

	if a.altOwnsKeyPath() {
		t.Fatal("with no modifiers held, the text path must keep its text")
	}

	pressAlt(a)
	if !a.altOwnsKeyPath() {
		t.Error("Option-composed text must be discarded while Alt is Meta")
	}

	a.mods.Key(key.Event{Name: key.NameAlt, State: key.Release})
	if a.altOwnsKeyPath() {
		t.Error("releasing Alt must hand text back to the text path")
	}
}

// TestAltMetaKeepsIMECommits guards the exemption: a CJK IME may hold Alt
// while committing, and that text is a genuine commit rather than an
// Option-glyph the key path is about to re-send.
func TestAltMetaKeepsIMECommits(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	pressAlt(a)
	a.ime.setComposing(key.Range{Start: 0, End: 2})

	if a.altOwnsKeyPath() {
		t.Error("text committed by an in-flight composition must not be dropped")
	}
}

// TestAltComposePolicyKeepsText covers the opt-out: a user who wants macOS
// Option-glyphs ("å", "Å") sets alt_is_meta = false, and then the text path
// must keep them.
func TestAltComposePolicyKeepsText(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	a.policy = input.Policy{AltIsMeta: false}
	pressAlt(a)

	if a.altOwnsKeyPath() {
		t.Error("with alt_is_meta = false, Option must still compose text")
	}
}

// TestAltPathsAgree is the invariant that actually prevents both bugs:
// exactly one of the two paths may claim any given Alt-chord. If IsTextKey
// says "not text" while onEdit keeps the text, the keystroke is duplicated;
// if both decline it, the keystroke is lost. Alt-Shift is included because
// that is the combination the user reported broken on macOS.
func TestAltPathsAgree(t *testing.T) {
	chords := []key.Event{
		{Name: "≈", Modifiers: key.ModAlt, State: key.Press},
		{Name: "A", Modifiers: key.ModAlt | key.ModShift, State: key.Press},
	}

	for _, altIsMeta := range []bool{true, false} {
		policy := input.Policy{AltIsMeta: altIsMeta}
		for _, ev := range chords {
			a := New(config.Default(), nil, Options{})
			a.policy = policy
			pressAlt(a)

			keyPathClaims := !policy.IsTextKey(ev)
			textPathDropped := a.altOwnsKeyPath()

			if keyPathClaims != textPathDropped {
				t.Errorf("AltIsMeta=%v, %+v: key path claims=%v but text path dropped=%v; the keystroke is %s",
					altIsMeta, ev, keyPathClaims, textPathDropped,
					map[bool]string{true: "duplicated", false: "lost"}[textPathDropped])
			}
		}
	}
}
