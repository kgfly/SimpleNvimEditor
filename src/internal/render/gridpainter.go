package render

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// Frame draws one full frame of the editor: the base grid, every placed
// split/float grid on top of it, and finally the cursor.
func Frame(gtx layout.Context, fonts Fonts, snap uistate.Snapshot) {
	defFg, defBg := snap.Highlight.DefaultColors()
	size := gtx.Constraints.Max
	paint.FillShape(gtx.Ops, defBg, clip.Rect(image.Rect(0, 0, size.X, size.Y)).Op())

	if base, ok := snap.Grids[1]; ok {
		drawGrid(gtx, fonts, snap.Highlight, base, image.Pt(0, 0))
	}
	for _, p := range snap.Windows {
		gv, ok := snap.Grids[p.GridID]
		if !ok {
			continue
		}
		origin := image.Pt(p.Col*fonts.Metrics.CellWidth, p.Row*fonts.Metrics.CellHeight)
		drawGrid(gtx, fonts, snap.Highlight, gv, origin)
	}

	drawCursor(gtx, fonts, snap, defFg)
}

// drawGrid paints every row of gv, offset by origin pixels, as a sequence of
// same-highlight cell "runs" (a background fill plus one text draw each).
func drawGrid(gtx layout.Context, fonts Fonts, hv uistate.HighlightView, gv uistate.GridView, origin image.Point) {
	cw, ch := fonts.Metrics.CellWidth, fonts.Metrics.CellHeight
	for row, cells := range gv.Data {
		y := origin.Y + row*ch
		col := 0
		for col < len(cells) {
			runStart := col
			hlID := cells[col].HlID
			var text []byte
			for col < len(cells) && cells[col].HlID == hlID {
				text = append(text, cells[col].Text...)
				col++
			}
			runLen := col - runStart
			if runLen == 0 {
				continue
			}
			x := origin.X + runStart*cw
			fg, bg := hv.Resolve(hlID)
			paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rect(x, y, x+runLen*cw, y+ch)).Op())
			drawText(gtx, fonts, x, y, string(text), fg)
		}
	}
}

// drawText paints one cell-run's text at the given top-left pixel position.
func drawText(gtx layout.Context, fonts Fonts, x, y int, text string, fg color.NRGBA) {
	if text == "" {
		return
	}
	off := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	m := op.Record(gtx.Ops)
	paint.ColorOp{Color: fg}.Add(gtx.Ops)
	material := m.Stop()
	labelWidget(gtx, fonts, text, material)
	off.Pop()
}

// drawCursor paints a block/beam/underline cursor per the active mode's
// ModeInfo. Text-under-cursor punch-through and blink/smooth-move animation
// are not implemented yet (see IMPLEMENTATION_PLAN.md Phase 2).
func drawCursor(gtx layout.Context, fonts Fonts, snap uistate.Snapshot, defFg color.NRGBA) {
	origin, ok := GridOrigin(snap, snap.Cursor.GridID, fonts.Metrics)
	if !ok {
		return
	}
	cw, ch := fonts.Metrics.CellWidth, fonts.Metrics.CellHeight
	x := origin.X + snap.Cursor.Col*cw
	y := origin.Y + snap.Cursor.Row*ch

	info := snap.CurrentModeInfo()
	col := cursorColor(snap.Highlight, info.AttrID, defFg)
	pct := info.CellPercentage
	if pct <= 0 {
		pct = 100
	}

	rect := image.Rect(x, y, x+cw, y+ch)
	switch info.CursorShape {
	case "horizontal":
		h := ch * pct / 100
		rect = image.Rect(x, y+ch-h, x+cw, y+ch)
	case "vertical":
		w := cw * pct / 100
		if w < 1 {
			w = 1
		}
		rect = image.Rect(x, y, x+w, y+ch)
	}
	paint.FillShape(gtx.Ops, col, clip.Rect(rect).Op())
}

// cursorColor resolves the fill color for the cursor block. Per `:h
// mode_info_set`, an attr_id of 0 means "use the default colors, swapped".
func cursorColor(hv uistate.HighlightView, attrID int, defFg color.NRGBA) color.NRGBA {
	if attrID == 0 {
		return defFg
	}
	_, bg := hv.Resolve(attrID)
	return bg
}

// GridOrigin returns the top-left pixel position of the given grid id
// within the window, or false if the grid isn't currently placed. It is
// exported because the IME needs the same mapping to tell the platform
// where the caret is on screen.
func GridOrigin(snap uistate.Snapshot, gridID int, m Metrics) (image.Point, bool) {
	if gridID == 1 {
		return image.Pt(0, 0), true
	}
	for _, p := range snap.Windows {
		if p.GridID == gridID {
			return image.Pt(p.Col*m.CellWidth, p.Row*m.CellHeight), true
		}
	}
	return image.Point{}, false
}
