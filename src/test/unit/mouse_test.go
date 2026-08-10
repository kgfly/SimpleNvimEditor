package unit_test

import (
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/kgfly/SimpleNvimEditor/internal/input"
)

func TestMouseButtonFor(t *testing.T) {
	cases := []struct {
		name    string
		buttons pointer.Buttons
		want    string
	}{
		{"primary", pointer.ButtonPrimary, "left"},
		{"secondary", pointer.ButtonSecondary, "right"},
		{"tertiary", pointer.ButtonTertiary, "middle"},
		{"none", 0, ""},
		{"primary wins when multiple bits set", pointer.ButtonPrimary | pointer.ButtonSecondary, "left"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := pointer.Event{Buttons: c.buttons}
			if got := input.MouseButtonFor(e); got != c.want {
				t.Fatalf("MouseButtonFor(%v) = %q, want %q", c.buttons, got, c.want)
			}
		})
	}
}

func TestMouseAction(t *testing.T) {
	cases := []struct {
		kind pointer.Kind
		want string
	}{
		{pointer.Press, "press"},
		{pointer.Release, "release"},
		{pointer.Drag, "drag"},
		{pointer.Move, ""},
		{pointer.Scroll, ""},
		{pointer.Cancel, ""},
	}
	for _, c := range cases {
		if got := input.MouseAction(c.kind); got != c.want {
			t.Errorf("MouseAction(%v) = %q, want %q", c.kind, got, c.want)
		}
	}
}

func TestScrollDirection(t *testing.T) {
	cases := []struct {
		name       string
		scroll     f32.Point
		wantAction string
		wantOK     bool
	}{
		{"scroll up (negative Y)", f32.Point{Y: -1}, "up", true},
		{"scroll down (positive Y)", f32.Point{Y: 1}, "down", true},
		{"scroll left (negative X)", f32.Point{X: -1}, "left", true},
		{"scroll right (positive X)", f32.Point{X: 1}, "right", true},
		{"no scroll", f32.Point{}, "", false},
		{"vertical takes precedence over horizontal", f32.Point{X: 5, Y: -5}, "up", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := pointer.Event{Scroll: c.scroll}
			action, ok := input.ScrollDirection(e)
			if action != c.wantAction || ok != c.wantOK {
				t.Fatalf("ScrollDirection(%v) = (%q, %v), want (%q, %v)", c.scroll, action, ok, c.wantAction, c.wantOK)
			}
		})
	}
}
