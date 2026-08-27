package render

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/widget"
)

// labelWidget draws txt at the top-left of gtx's current transform, using
// fonts and pre-resolved material (typically a solid color, see
// paint.ColorOp), without wrapping or stretching to the constraints.
func labelWidget(gtx layout.Context, fonts Fonts, txt string, material op.CallOp) {
	widget.Label{MaxLines: 1}.Layout(unconstrained(gtx), fonts.Shaper, fonts.Face, fonts.Size, txt, material)
}

// SolidMaterial records a reusable paint material for a solid color. It is
// a small convenience used outside the hot per-cell path (e.g. by callers
// that just want to draw one string, like future chrome widgets).
func SolidMaterial(ops *op.Ops, c color.NRGBA) op.CallOp {
	m := op.Record(ops)
	paint.ColorOp{Color: c}.Add(ops)
	return m.Stop()
}

// glyphKey identifies one drawable cell: its text and its foreground color.
type glyphKey struct {
	text string
	fg   color.NRGBA
}

// glyphCache memoizes the drawing commands for each distinct glyph within a
// single frame.
//
// A terminal grid is enormously repetitive: a full screen is tens of
// thousands of cells drawn from only a few dozen distinct character/color
// combinations. Shaping each cell separately repeats that expensive work
// over and over, and it dominates the cost of a frame -- measurably worse
// on a low-DPI monitor, where smaller cells mean *more* of them for the
// same physical screen (a 3440x1378 external panel yields a 491x91 grid =
// ~45k cells, versus ~23k on the Retina built-in).
//
// Recording each glyph into a macro once and replaying it at each position
// keeps the output pixel-identical: every cell is still placed at its own
// exact grid coordinate, so none of the sub-pixel drift that per-cell
// drawing exists to avoid can creep back in.
//
// The cache lives for exactly one frame, because the macros it stores point
// into that frame's op.Ops buffer.
type glyphCache struct {
	ops   *op.Ops
	fonts Fonts
	calls map[glyphKey]op.CallOp
}

func newGlyphCache(gtx layout.Context, fonts Fonts) *glyphCache {
	return &glyphCache{
		ops:   gtx.Ops,
		fonts: fonts,
		calls: make(map[glyphKey]op.CallOp),
	}
}

// get returns the recorded drawing commands for one glyph, shaping it on
// first use and reusing the recording thereafter.
func (c *glyphCache) get(gtx layout.Context, text string, fg color.NRGBA) op.CallOp {
	key := glyphKey{text: text, fg: fg}
	if call, ok := c.calls[key]; ok {
		return call
	}
	glyph := op.Record(c.ops)
	material := SolidMaterial(c.ops, fg)
	labelWidget(gtx, c.fonts, text, material)
	call := glyph.Stop()
	c.calls[key] = call
	return call
}
