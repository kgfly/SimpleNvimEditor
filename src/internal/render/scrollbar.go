package render

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// Scrollbar is the on-screen geometry of one window's native scrollbar, in
// window pixels. It is drawn over the window's rightmost column and is also
// what the app hit-tests clicks and drags against.
type Scrollbar struct {
	GridID int
	// Win is the Nvim window handle to scroll.
	Win int
	// Track spans the window's full height; Thumb is the visible part.
	Track image.Rectangle
	Thumb image.Rectangle
	// LineCount and Visible are the buffer lines in total and on screen,
	// as of this frame.
	LineCount int
	Visible   int
}

// Scrollbars lays out a bar for every split window whose buffer has more
// lines than fit on screen. Floats (popups, pickers, scrollbar plugins) and
// windows whose content all fits get none.
func Scrollbars(snap uistate.Snapshot, m Metrics) []Scrollbar {
	cw, ch := m.CellWidth, m.CellHeight
	if cw == 0 || ch == 0 {
		return nil
	}
	width := max(4, cw*2/3)
	var bars []Scrollbar
	for _, p := range snap.Windows {
		if p.Float || p.Hidden || p.Width < 2 || p.Height < 1 {
			continue
		}
		vp, ok := snap.Viewports[p.GridID]
		if !ok || vp.LineCount <= 0 {
			continue
		}
		visible := vp.Visible()
		if vp.Topline == 0 && visible >= vp.LineCount {
			continue
		}
		right := (p.Col + p.Width) * cw
		track := image.Rect(right-width, p.Row*ch, right, (p.Row+p.Height)*ch)
		bars = append(bars, Scrollbar{
			GridID:    p.GridID,
			Win:       vp.Win,
			Track:     track,
			Thumb:     thumbRect(track, vp.Topline, visible, vp.LineCount, ch),
			LineCount: vp.LineCount,
			Visible:   visible,
		})
	}
	return bars
}

// thumbRect sizes the thumb by the visible fraction of the buffer (but no
// shorter than one cell, so it stays grabbable) and places it so it touches
// the bottom exactly when the last line is on screen.
func thumbRect(track image.Rectangle, topline, visible, lineCount, minHeight int) image.Rectangle {
	h := track.Dy()
	thumbH := min(h, max(minHeight, h*visible/max(1, lineCount)))
	span := lineCount - visible
	y := 0
	if span > 0 {
		y = (h - thumbH) * min(topline, span) / span
	}
	return image.Rect(track.Min.X, track.Min.Y+y, track.Max.X, track.Min.Y+y+thumbH)
}

// ToplineAt inverts thumbRect: it returns the 0-based topline that puts the
// thumb's top edge at window pixel y. Positions at or past the end of the
// track return LineCount, meaning "scroll to the bottom".
func (b Scrollbar) ToplineAt(y int) int {
	travel := b.Track.Dy() - b.Thumb.Dy()
	span := b.LineCount - b.Visible
	off := y - b.Track.Min.Y
	if travel <= 0 || span <= 0 || off <= 0 {
		return 0
	}
	if off >= travel {
		return b.LineCount
	}
	return (off*span + travel/2) / travel
}

// drawScrollbar paints b's track and thumb in shades between the default
// background and foreground, so the bar reads on dark and light themes
// alike. active brightens the thumb while it is being dragged.
func drawScrollbar(gtx layout.Context, hv uistate.HighlightView, b Scrollbar, active bool) {
	fg, bg := hv.DefaultColors()
	thumbMix := 0.45
	if active {
		thumbMix = 0.65
	}
	paint.FillShape(gtx.Ops, mix(bg, fg, 0.15), clip.Rect(b.Track).Op())
	paint.FillShape(gtx.Ops, mix(bg, fg, thumbMix), clip.Rect(b.Thumb).Op())
}

// mix blends from a toward b by t in [0, 1].
func mix(a, b color.NRGBA, t float64) color.NRGBA {
	lerp := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return color.NRGBA{R: lerp(a.R, b.R), G: lerp(a.G, b.G), B: lerp(a.B, b.B), A: 255}
}
