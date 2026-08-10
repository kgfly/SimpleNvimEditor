package unit_test

import (
	"image"
	"testing"

	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/op"
	"gioui.org/op/clip"

	appkg "github.com/kgfly/SimpleNvimEditor/internal/app"
	inputpkg "github.com/kgfly/SimpleNvimEditor/internal/input"
)

// deliverKey pushes one key event through a real Gio input.Router using the
// exact filters production registers, and reports the event the app would
// actually receive.
//
// This guards the entire input path rather than just the encoder. A
// key.Filter whose Optional modifier set is unset makes Gio silently
// discard every modified keypress, which encoder-only tests cannot catch.
func deliverKey(t *testing.T, ev key.Event) (key.Event, bool) {
	t.Helper()

	tag := new(int)
	var r input.Router
	filters := appkg.InputFilters(tag)

	// Rebuild the op tree each frame, mirroring App.handleInput.
	mkops := func() *op.Ops {
		ops := new(op.Ops)
		area := clip.Rect(image.Rectangle{Max: image.Pt(800, 600)}).Push(ops)
		event.Op(ops, tag)
		area.Pop()
		return ops
	}

	drain := func() []key.Event {
		var got []key.Event
		for {
			e, found := r.Event(filters...)
			if !found {
				return got
			}
			if ke, isKey := e.(key.Event); isKey {
				got = append(got, ke)
			}
		}
	}

	// Router.Frame swaps filter/nextFilter, so filters registered during
	// one frame only take effect on the next. Run two warm-up frames to
	// reach the steady state a running app is always in, then queue.
	drain()
	r.Source().Execute(key.FocusCmd{Tag: tag})
	r.Frame(mkops())

	drain()
	r.Frame(mkops())

	drain()
	r.Queue(ev)
	for _, got := range drain() {
		if got.State == key.Press {
			return got, true
		}
	}
	return key.Event{}, false
}

// TestInputFiltersDeliverModifiedKeys is the regression test for modified
// keys never reaching Nvim: ":" could not be typed (so no command line) and
// "<C-w>s"/"<C-w>v" did nothing (so no window splits).
func TestInputFiltersDeliverModifiedKeys(t *testing.T) {
	cases := []struct {
		desc string
		ev   key.Event
		want string
	}{
		{
			desc: "colon via shift opens the command line",
			ev:   key.Event{Name: ":", Modifiers: key.ModShift, State: key.Press},
			want: ":",
		},
		{
			desc: "ctrl-w is the window prefix",
			ev:   key.Event{Name: "W", Modifiers: key.ModCtrl, State: key.Press},
			want: "<C-w>",
		},
		{
			desc: "unmodified key still works",
			ev:   key.Event{Name: "S", State: key.Press},
			want: "s",
		},
		{
			desc: "shifted letter stays uppercase",
			ev:   key.Event{Name: "S", Modifiers: key.ModShift, State: key.Press},
			want: "S",
		},
		{
			desc: "alt-modified key",
			ev:   key.Event{Name: "X", Modifiers: key.ModAlt, State: key.Press},
			want: "<A-x>",
		},
		{
			// Shift is already encoded in the glyph ("A" not "a"), so
			// an extra "S-" would make Nvim see a different key.
			desc: "ctrl-shift combination",
			ev:   key.Event{Name: "A", Modifiers: key.ModCtrl | key.ModShift, State: key.Press},
			want: "<C-A>",
		},
		{
			desc: "named key with a modifier",
			ev:   key.Event{Name: key.NameEscape, Modifiers: key.ModCtrl, State: key.Press},
			want: "<C-Esc>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			got, ok := deliverKey(t, tc.ev)
			if !ok {
				t.Fatalf("event %+v was filtered out by Gio and never reached the app", tc.ev)
			}
			if encoded := inputpkg.EncodeKey(got); encoded != tc.want {
				t.Errorf("EncodeKey(delivered %+v) = %q, want %q", got, encoded, tc.want)
			}
		})
	}
}

// TestCtrlWSplitSequence walks the full "<C-w>s" and "<C-w>v" sequences the
// user reported as broken, asserting each keystroke survives filtering and
// encodes to what Nvim expects.
func TestCtrlWSplitSequence(t *testing.T) {
	sequences := map[string][]struct {
		ev   key.Event
		want string
	}{
		"horizontal split": {
			{key.Event{Name: "W", Modifiers: key.ModCtrl, State: key.Press}, "<C-w>"},
			{key.Event{Name: "S", State: key.Press}, "s"},
		},
		"vertical split": {
			{key.Event{Name: "W", Modifiers: key.ModCtrl, State: key.Press}, "<C-w>"},
			{key.Event{Name: "V", State: key.Press}, "v"},
		},
	}

	for name, steps := range sequences {
		t.Run(name, func(t *testing.T) {
			for i, step := range steps {
				got, ok := deliverKey(t, step.ev)
				if !ok {
					t.Fatalf("step %d: event %+v never reached the app", i, step.ev)
				}
				if encoded := inputpkg.EncodeKey(got); encoded != step.want {
					t.Fatalf("step %d: got %q, want %q", i, encoded, step.want)
				}
			}
		})
	}
}
