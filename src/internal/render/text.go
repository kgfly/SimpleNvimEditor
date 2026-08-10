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
