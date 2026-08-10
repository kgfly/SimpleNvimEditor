package uistate

import "image/color"

// RGB is a simple opaque color; -1 (Unset) means "use the default
// foreground/background", matching how Nvim's `hl_attr_define` omits a
// field entirely rather than sending a sentinel value.
type RGB struct {
	set     bool
	r, g, b uint8
}

// NRGBA converts to Gio/Go's standard color type, falling back to def when
// the color was never set (e.g. an attribute that doesn't override fg/bg).
func (c RGB) NRGBA(def color.NRGBA) color.NRGBA {
	if !c.set {
		return def
	}
	return color.NRGBA{R: c.r, G: c.g, B: c.b, A: 0xff}
}

func rgbFromInt(v interface{}) RGB {
	n := toInt(v)
	return RGB{set: true, r: uint8(n >> 16), g: uint8(n >> 8), b: uint8(n)}
}

// HlAttr is one entry from `hl_attr_define`: the decoded rgb_attr map.
type HlAttr struct {
	Foreground RGB
	Background RGB
	Special    RGB
	Reverse    bool
	Bold       bool
	Italic     bool
	Underline  bool
	Undercurl  bool
	Strike     bool
}

// HighlightTable tracks the highlight attribute table and the two default
// colors (id 0) that everything else falls back to.
type HighlightTable struct {
	attrs       map[int]HlAttr
	defaultFg   color.NRGBA
	defaultBg   color.NRGBA
	defaultSpec color.NRGBA
}

func newHighlightTable() *HighlightTable {
	return &HighlightTable{
		attrs:     make(map[int]HlAttr),
		defaultFg: color.NRGBA{R: 0xd0, G: 0xd0, B: 0xd0, A: 0xff},
		defaultBg: color.NRGBA{R: 0x1e, G: 0x1e, B: 0x1e, A: 0xff},
	}
}

func (h *HighlightTable) applyDefaultColorsSet(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 3 {
			continue
		}
		h.defaultFg = rgbFromInt(t[0]).NRGBA(h.defaultFg)
		h.defaultBg = rgbFromInt(t[1]).NRGBA(h.defaultBg)
		h.defaultSpec = rgbFromInt(t[2]).NRGBA(h.defaultSpec)
	}
}

func (h *HighlightTable) applyHlAttrDefine(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 2 {
			continue
		}
		id := toInt(t[0])
		rgbAttr := toMap(t[1])

		var attr HlAttr
		if v, ok := rgbAttr["foreground"]; ok {
			attr.Foreground = rgbFromInt(v)
		}
		if v, ok := rgbAttr["background"]; ok {
			attr.Background = rgbFromInt(v)
		}
		if v, ok := rgbAttr["special"]; ok {
			attr.Special = rgbFromInt(v)
		}
		attr.Reverse = toBool(rgbAttr["reverse"])
		attr.Bold = toBool(rgbAttr["bold"])
		attr.Italic = toBool(rgbAttr["italic"])
		attr.Underline = toBool(rgbAttr["underline"])
		attr.Undercurl = toBool(rgbAttr["undercurl"])
		attr.Strike = toBool(rgbAttr["strikethrough"])

		h.attrs[id] = attr
	}
}

// HighlightView is the read-only, per-frame view of the highlight table
// handed to the renderer. Resolve turns an hl_id into concrete fg/bg colors,
// already accounting for the `reverse` attribute and the id-0 fallback.
type HighlightView struct {
	attrs     map[int]HlAttr
	defaultFg color.NRGBA
	defaultBg color.NRGBA
}

func (h *HighlightTable) view() HighlightView {
	cp := make(map[int]HlAttr, len(h.attrs))
	for k, v := range h.attrs {
		cp[k] = v
	}
	return HighlightView{attrs: cp, defaultFg: h.defaultFg, defaultBg: h.defaultBg}
}

// Resolve returns the foreground and background color to paint a cell with
// the given highlight id.
func (v HighlightView) Resolve(hlID int) (fg, bg color.NRGBA) {
	fg, bg = v.defaultFg, v.defaultBg
	attr, ok := v.attrs[hlID]
	if !ok {
		return fg, bg
	}
	fg = attr.Foreground.NRGBA(fg)
	bg = attr.Background.NRGBA(bg)
	if attr.Reverse {
		fg, bg = bg, fg
	}
	return fg, bg
}

// Attr returns the raw attribute record for the given id, for callers that
// need style flags (bold/italic/...) in addition to colors.
func (v HighlightView) Attr(hlID int) HlAttr {
	return v.attrs[hlID]
}

// DefaultColors returns the id-0 fallback colors directly, used to draw
// areas of the window that have no grid content (e.g. letterboxing).
func (v HighlightView) DefaultColors() (fg, bg color.NRGBA) {
	return v.defaultFg, v.defaultBg
}
