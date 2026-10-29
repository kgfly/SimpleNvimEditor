package render

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"

	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// TestFrameDrawsScrollbarTrackAndThumb rasterizes a window with an
// overflowing buffer and checks the track is a visible shade (not the
// background) and the thumb is a lighter one.
func TestFrameDrawsScrollbarTrackAndThumb(t *testing.T) {
	px := image.Pt(200, 200)
	fonts := testFonts(t, 1, px)
	cw, ch := fonts.Metrics.CellWidth, fonts.Metrics.CellHeight
	cols, rows := px.X/cw, px.Y/ch

	s := uistate.New()
	s.Apply([][]interface{}{
		{"default_colors_set", []interface{}{int64(0xffffff), int64(0x000000), int64(0xffffff), int64(0), int64(0)}},
		{"grid_resize", []interface{}{int64(2), int64(cols), int64(rows)}},
		{"win_pos", []interface{}{int64(2), int64(1000), int64(0), int64(0), int64(cols), int64(rows)}},
		{"win_viewport", []interface{}{int64(2), int64(1000), int64(0), int64(rows), int64(0), int64(0), int64(rows * 4), int64(0)}},
	})
	snap := s.Snapshot()
	bars := Scrollbars(snap, fonts.Metrics)
	if len(bars) != 1 {
		t.Fatalf("got %d bars, want 1", len(bars))
	}
	b := bars[0]

	img := rasterize(t, px, func(gtx layout.Context) {
		FrameWith(gtx, fonts, snap, FrameOptions{Scrollbars: bars})
	})
	black := color.NRGBA{A: 255}
	track := mix(black, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, 0.15)
	thumb := mix(black, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, 0.45)

	below := image.Rect(b.Track.Min.X, b.Thumb.Max.Y, b.Track.Max.X, b.Track.Max.Y)
	if n := countColor(img, below, track); n != below.Dx()*below.Dy() {
		t.Errorf("track below the thumb has %d/%d track-colored pixels", n, below.Dx()*below.Dy())
	}
	if n := countColor(img, b.Thumb, thumb); n != b.Thumb.Dx()*b.Thumb.Dy() {
		t.Errorf("thumb has %d/%d thumb-colored pixels", n, b.Thumb.Dx()*b.Thumb.Dy())
	}

	plain := rasterize(t, px, func(gtx layout.Context) { Frame(gtx, fonts, snap) })
	if n := countColor(plain, b.Track, track) + countColor(plain, b.Track, thumb); n != 0 {
		t.Errorf("Frame without scrollbars painted %d scrollbar pixels", n)
	}
}
