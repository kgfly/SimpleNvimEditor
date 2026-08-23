// Package render draws a uistate.Snapshot into a Gio frame. It is kept
// deliberately simple for the MVP: no glyph caching, no smooth-scroll or
// cursor animation yet (see IMPLEMENTATION_PLAN.md Phase 2+). Every frame
// walks the grids and paints cell-runs directly.
package render

import (
	"image"
	"strings"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
)

// Metrics describes the fixed pixel size of one terminal cell, derived from
// the active monospace font.
type Metrics struct {
	CellWidth  int
	CellHeight int
	Baseline   int
}

// Fonts bundles the shaper and the font descriptor rows should be drawn
// with, plus the derived cell Metrics.
type Fonts struct {
	Shaper  *text.Shaper
	Face    font.Font
	Size    unit.Sp
	Metrics Metrics
}

// NewShaper builds a text shaper according to the configuration: either the
// bundled, fully self-contained Go Mono typeface (the default, chosen so
// the app renders identically regardless of what's installed on the host),
// or the system font matcher when the user opts in. System-font mode still
// includes the bundled Go Mono as a fallback so text never disappears if a
// configured family can't be found.
func NewShaper(cfg config.EditorConfig) *text.Shaper {
	if cfg.UseSystemFonts {
		return text.NewShaper(text.WithCollection(gofont.Collection()))
	}
	return text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
}

// FontFace returns the font.Font to request from the shaper for grid text.
func FontFace(cfg config.EditorConfig) font.Font {
	if cfg.UseSystemFonts {
		family, weight := parseFontFamily(cfg.FontFamily)
		return font.Font{Typeface: font.Typeface(family), Weight: weight}
	}
	return font.Font{Typeface: "Go Mono"}
}

func parseFontFamily(name string) (string, font.Weight) {
	weights := map[string]font.Weight{
		"Thin":       font.Thin,
		"ExtraLight": font.ExtraLight,
		"Light":      font.Light,
		"Medium":     font.Medium,
		"SemiBold":   font.SemiBold,
		"Bold":       font.Bold,
		"ExtraBold":  font.ExtraBold,
		"Black":      font.Black,
	}
	for suffix, w := range weights {
		if strings.HasSuffix(name, " "+suffix) {
			return strings.TrimSuffix(name, " "+suffix), w
		}
	}
	return name, font.Normal
}

// Measure lays out a probe string to derive the pixel dimensions of one grid
// cell for the given font and size. It must be called with an
// unconstrained-width context (see unconstrained) so the shaper reports the
// text's natural size instead of stretching to fill the frame.
func Measure(gtx layout.Context, shaper *text.Shaper, face font.Font, size unit.Sp) Metrics {
	const probe = "MMMMMMMMMMMMMMMMMMMM" // 20 cells; averages out hinting rounding
	mgtx := unconstrained(gtx)
	var ops op.Ops
	mgtx.Ops = &ops
	dims := widget.Label{MaxLines: 1}.Layout(mgtx, shaper, face, size, probe, op.CallOp{})
	w := dims.Size.X / len(probe)
	if w < 1 {
		w = 1
	}
	h := dims.Size.Y
	if h < 1 {
		h = 1
	}
	return Metrics{CellWidth: w, CellHeight: h, Baseline: dims.Baseline}
}

// unconstrained returns a copy of gtx whose constraints impose no minimum
// size, so that widgets (in particular widget.Label) report their natural
// size instead of stretching to fill the parent.
func unconstrained(gtx layout.Context) layout.Context {
	gtx.Constraints = layout.Constraints{Max: image.Pt(1<<20, 1<<20)}
	return gtx
}
