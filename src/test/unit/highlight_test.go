package unit_test

import (
	"image/color"
	"testing"

	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// rgbInt packs r,g,b into the single 24-bit integer Nvim sends for colors
// (see `:h nvim_ui_attach` rgb_attr / default_colors_set).
func rgbInt(r, g, b uint8) int {
	return int(r)<<16 | int(g)<<8 | int(b)
}

func TestDefaultColorsSet(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("default_colors_set", args(
		rgbInt(0x11, 0x22, 0x33), // foreground
		rgbInt(0x44, 0x55, 0x66), // background
		rgbInt(0x77, 0x88, 0x99), // special
	))))

	fg, bg := s.Snapshot().Highlight.DefaultColors()
	wantFg := color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff}
	wantBg := color.NRGBA{R: 0x44, G: 0x55, B: 0x66, A: 0xff}
	if fg != wantFg {
		t.Fatalf("default fg = %+v, want %+v", fg, wantFg)
	}
	if bg != wantBg {
		t.Fatalf("default bg = %+v, want %+v", bg, wantBg)
	}
}

func TestHlAttrDefineResolvesForegroundAndBackground(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("hl_attr_define", args(
		42,
		map[string]interface{}{
			"foreground": rgbInt(0xff, 0x00, 0x00),
			"background": rgbInt(0x00, 0xff, 0x00),
		},
		[]interface{}{}, // cterm_attr, unused by our decoder
		[]interface{}{}, // info, unused by our decoder
	))))

	fg, bg := s.Snapshot().Highlight.Resolve(42)
	if want := (color.NRGBA{R: 0xff, A: 0xff}); fg != want {
		t.Fatalf("fg = %+v, want %+v", fg, want)
	}
	if want := (color.NRGBA{G: 0xff, A: 0xff}); bg != want {
		t.Fatalf("bg = %+v, want %+v", bg, want)
	}
}

func TestHlAttrDefineReverseSwapsColors(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("default_colors_set", args(rgbInt(1, 2, 3), rgbInt(4, 5, 6), 0)),
		ev("hl_attr_define", args(
			7,
			map[string]interface{}{
				"foreground": rgbInt(10, 20, 30),
				"background": rgbInt(40, 50, 60),
				"reverse":    true,
			},
			[]interface{}{}, []interface{}{},
		)),
	))

	fg, bg := s.Snapshot().Highlight.Resolve(7)
	wantFg := color.NRGBA{R: 40, G: 50, B: 60, A: 0xff}
	wantBg := color.NRGBA{R: 10, G: 20, B: 30, A: 0xff}
	if fg != wantFg {
		t.Fatalf("reversed fg = %+v, want %+v", fg, wantFg)
	}
	if bg != wantBg {
		t.Fatalf("reversed bg = %+v, want %+v", bg, wantBg)
	}
}

func TestHlAttrDefineUnsetFieldFallsBackToDefault(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("default_colors_set", args(rgbInt(9, 9, 9), rgbInt(1, 1, 1), 0)),
		// Only sets foreground; background should fall back to default.
		ev("hl_attr_define", args(
			3,
			map[string]interface{}{"foreground": rgbInt(200, 200, 200)},
			[]interface{}{}, []interface{}{},
		)),
	))

	fg, bg := s.Snapshot().Highlight.Resolve(3)
	if want := (color.NRGBA{R: 200, G: 200, B: 200, A: 0xff}); fg != want {
		t.Fatalf("fg = %+v, want %+v", fg, want)
	}
	if want := (color.NRGBA{R: 1, G: 1, B: 1, A: 0xff}); bg != want {
		t.Fatalf("bg fallback = %+v, want default bg %+v", bg, want)
	}
}

func TestHlAttrDefineBoolFlags(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("hl_attr_define", args(
		1,
		map[string]interface{}{
			"bold":          true,
			"italic":        true,
			"underline":     true,
			"undercurl":     true,
			"strikethrough": true,
		},
		[]interface{}{}, []interface{}{},
	))))

	attr := s.Snapshot().Highlight.Attr(1)
	if !attr.Bold || !attr.Italic || !attr.Underline || !attr.Undercurl || !attr.Strike {
		t.Fatalf("attr flags = %+v, want all true", attr)
	}
}

func TestResolveUnknownHlIDReturnsDefaults(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("default_colors_set", args(rgbInt(5, 5, 5), rgbInt(6, 6, 6), 0))))

	fg, bg := s.Snapshot().Highlight.Resolve(999)
	if want := (color.NRGBA{R: 5, G: 5, B: 5, A: 0xff}); fg != want {
		t.Fatalf("fg for unknown id = %+v, want default %+v", fg, want)
	}
	if want := (color.NRGBA{R: 6, G: 6, B: 6, A: 0xff}); bg != want {
		t.Fatalf("bg for unknown id = %+v, want default %+v", bg, want)
	}
}
