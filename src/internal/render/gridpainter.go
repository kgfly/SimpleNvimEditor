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

// HoverLink identifies a visible URL cell range to decorate for the current
// frame. EndCol is exclusive. More holds the link's other rows when it
// soft-wraps across several grid rows.
type HoverLink struct {
	Active   bool
	GridID   int
	Row      int
	StartCol int
	EndCol   int
	More     []HoverSpan
}

// HoverSpan is one row's [StartCol, EndCol) slice of a wrapped hover link.
type HoverSpan struct {
	Row      int
	StartCol int
	EndCol   int
}

// FrameOptions are the per-frame extras Frame draws on top of the grids.
type FrameOptions struct {
	Hover HoverLink
	// Scrollbars are drawn over their windows; nil draws none.
	Scrollbars []Scrollbar
	// ActiveScrollbar is the grid whose thumb is being dragged, or 0.
	ActiveScrollbar int
}

// Frame draws one full frame of the editor: the base grid, every placed
// split/float grid on top of it, and finally the cursor.
func Frame(gtx layout.Context, fonts Fonts, snap uistate.Snapshot, hovered ...HoverLink) {
	var opts FrameOptions
	if len(hovered) > 0 {
		opts.Hover = hovered[0]
	}
	FrameWith(gtx, fonts, snap, opts)
}

// FrameWith is Frame with all options. Each scrollbar is painted right
// after its window, so floats that overlap it still cover it.
func FrameWith(gtx layout.Context, fonts Fonts, snap uistate.Snapshot, opts FrameOptions) {
	_, defBg := snap.Highlight.DefaultColors()
	size := gtx.Constraints.Max
	paint.FillShape(gtx.Ops, defBg, clip.Rect(image.Rect(0, 0, size.X, size.Y)).Op())
	hover := opts.Hover

	// One cache per frame, shared by every grid: splits and floats draw
	// the same characters as the base grid, so they hit the same entries.
	glyphs := newGlyphCache(gtx, fonts)

	if base, ok := snap.Grids[1]; ok {
		drawGrid(gtx, fonts, glyphs, snap.Highlight, base, image.Pt(0, 0), hover)
	}
	for _, p := range snap.Windows {
		gv, ok := snap.Grids[p.GridID]
		if !ok {
			continue
		}
		origin := image.Pt(p.Col*fonts.Metrics.CellWidth, p.Row*fonts.Metrics.CellHeight)
		drawGrid(gtx, fonts, glyphs, snap.Highlight, gv, origin, hover)
		for _, b := range opts.Scrollbars {
			if b.GridID == p.GridID {
				drawScrollbar(gtx, snap.Highlight, b, b.GridID == opts.ActiveScrollbar)
			}
		}
	}

	drawCursor(gtx, fonts, glyphs, snap)
}

// drawGrid paints every row of gv, offset by origin pixels. Cells are
// grouped into same-highlight runs for the background fill, but glyphs are
// drawn one cell at a time so text stays locked to the grid.
func drawGrid(gtx layout.Context, fonts Fonts, glyphs *glyphCache, hv uistate.HighlightView, gv uistate.GridView, origin image.Point, hover HoverLink) {
	cw, ch := fonts.Metrics.CellWidth, fonts.Metrics.CellHeight
	for row, cells := range gv.Data {
		y := origin.Y + row*ch
		col := 0
		for col < len(cells) {
			runStart := col
			hlID := cells[col].HlID
			for col < len(cells) && cells[col].HlID == hlID {
				col++
			}
			runLen := col - runStart
			if runLen == 0 {
				continue
			}
			x := origin.X + runStart*cw
			fg, bg := hv.Resolve(hlID)
			paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rect(x, y, x+runLen*cw, y+ch)).Op())

			// Draw each cell at its own exact grid position instead of
			// shaping the whole run as one string. A font's real advance is
			// fractional and rarely equals the integer CellWidth the grid is
			// laid out on, so a shaped run drifts further right with every
			// glyph; over a long line that accumulates into whole columns of
			// error, visibly clipping characters at run boundaries.
			for i, cell := range cells[runStart:col] {
				if cell.Text == "" || cell.Text == " " {
					continue
				}
				drawText(gtx, glyphs, x+i*cw, y, cell.Text, fg)
			}
		}
		if hover.Active && hover.GridID == gv.ID {
			if hover.Row == row {
				drawHoverUnderline(gtx, fonts.Metrics, hv, cells, origin, HoverSpan{Row: row, StartCol: hover.StartCol, EndCol: hover.EndCol})
			}
			for _, span := range hover.More {
				if span.Row == row {
					drawHoverUnderline(gtx, fonts.Metrics, hv, cells, origin, span)
				}
			}
		}
	}
}

func drawHoverUnderline(gtx layout.Context, metrics Metrics, hv uistate.HighlightView, cells []uistate.Cell, origin image.Point, hover HoverSpan) {
	start := max(0, hover.StartCol)
	end := min(len(cells), hover.EndCol)
	if start >= end {
		return
	}
	// Gio's Dimensions.Baseline is measured up from the bottom edge, not
	// down from the top. Convert it to a top-relative baseline before
	// placing the stroke one pixel into the descent area.
	baseline := metrics.CellHeight - metrics.Baseline
	y := origin.Y + hover.Row*metrics.CellHeight + min(baseline+1, metrics.CellHeight-1)
	bottom := origin.Y + (hover.Row+1)*metrics.CellHeight
	thickness := max(1, metrics.CellHeight/16)
	for col := start; col < end; col++ {
		fg, _ := hv.Resolve(cells[col].HlID)
		x := origin.X + col*metrics.CellWidth
		paint.FillShape(gtx.Ops, fg, clip.Rect(image.Rect(x, y, x+metrics.CellWidth, min(y+thickness, bottom))).Op())
	}
}

// drawText paints one cell's text at the given top-left pixel position,
// reusing the frame's shaped recording of that glyph.
func drawText(gtx layout.Context, glyphs *glyphCache, x, y int, text string, fg color.NRGBA) {
	if text == "" {
		return
	}
	call := glyphs.get(gtx, text, fg)
	off := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	off.Pop()
}

// drawCursor paints a block/beam/underline cursor per the active mode's
// ModeInfo. A block cursor redraws the character beneath it in the cursor's
// text color, like a terminal's inverse-video cursor, so the character stays
// readable. Blink/smooth-move animation is not implemented.
func drawCursor(gtx layout.Context, fonts Fonts, glyphs *glyphCache, snap uistate.Snapshot) {
	origin, ok := GridOrigin(snap, snap.Cursor.GridID, fonts.Metrics)
	if !ok {
		return
	}
	info := snap.CurrentModeInfo()
	// A fully blended cursor is how plugins (e.g. smear-cursor) hide the
	// real cursor while drawing their own.
	if info.AttrID != 0 && snap.Highlight.Attr(info.AttrID).Blend >= 100 {
		return
	}

	cw, ch := fonts.Metrics.CellWidth, fonts.Metrics.CellHeight
	x := origin.X + snap.Cursor.Col*cw
	y := origin.Y + snap.Cursor.Row*ch

	cell, cellWidth := cursorCell(snap)
	blockCol, textCol := cursorColors(snap.Highlight, info.AttrID, cell.HlID)
	pct := info.CellPercentage
	if pct <= 0 {
		pct = 100
	}

	rect := image.Rect(x, y, x+cw*cellWidth, y+ch)
	switch info.CursorShape {
	case "horizontal":
		h := ch * pct / 100
		rect = image.Rect(x, y+ch-h, x+cw*cellWidth, y+ch)
	case "vertical":
		w := cw * pct / 100
		if w < 1 {
			w = 1
		}
		rect = image.Rect(x, y, x+w, y+ch)
	}
	paint.FillShape(gtx.Ops, blockCol, clip.Rect(rect).Op())

	if info.CursorShape != "horizontal" && info.CursorShape != "vertical" && cell.Text != "" && cell.Text != " " {
		drawText(gtx, glyphs, x, y, cell.Text, textCol)
	}
}

// cursorCell returns the cell under the cursor and how many columns it
// spans (2 for a double-width character, whose second cell has empty text).
func cursorCell(snap uistate.Snapshot) (uistate.Cell, int) {
	gv, ok := snap.Grids[snap.Cursor.GridID]
	if !ok || snap.Cursor.Row < 0 || snap.Cursor.Row >= len(gv.Data) {
		return uistate.Cell{}, 1
	}
	cells := gv.Data[snap.Cursor.Row]
	col := snap.Cursor.Col
	if col < 0 || col >= len(cells) {
		return uistate.Cell{}, 1
	}
	if cells[col].Text != "" && col+1 < len(cells) && cells[col+1].Text == "" {
		return cells[col], 2
	}
	return cells[col], 1
}

// cursorColors resolves the cursor's fill color and the color of the text
// drawn on top of it. Per `:h mode_info_set`, an attr_id of 0 means "swap the
// colors", which we apply to the cell under the cursor (as terminals do).
// Colors the cursor highlight leaves unset fall back the same way, so e.g.
// `Cursor guibg=red` still gets readable text.
func cursorColors(hv uistate.HighlightView, attrID, cellHlID int) (block, text color.NRGBA) {
	cellFg, cellBg := hv.Resolve(cellHlID)
	if attrID == 0 {
		return cellFg, cellBg
	}
	attr := hv.Attr(attrID)
	if attr.Reverse {
		return attr.Foreground.NRGBA(cellFg), attr.Background.NRGBA(cellBg)
	}
	return attr.Background.NRGBA(cellFg), attr.Foreground.NRGBA(cellBg)
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
