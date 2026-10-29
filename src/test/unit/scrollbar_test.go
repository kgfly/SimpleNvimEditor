package unit_test

import (
	"image"
	"testing"

	"github.com/kgfly/SimpleNvimEditor/internal/render"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// rpcWindow stands in for go-client's nvim.Window: the RPC layer decodes
// window handles into a named integer type, not a plain int64.
type rpcWindow int

func TestWinViewportRecordsScrollState(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("win_pos", args(2, rpcWindow(1000), 0, 0, 80, 30)),
		// win_viewport: [grid, win, topline, botline, curline, curcol, line_count, scroll_delta]
		ev("win_viewport", args(2, rpcWindow(1000), 10, 40, 12, 0, 300, 10)),
	))

	got := s.Snapshot().Viewports[2]
	want := uistate.Viewport{Win: 1000, Topline: 10, Botline: 40, LineCount: 300}
	if got != want {
		t.Fatalf("viewport = %+v, want %+v", got, want)
	}
	if got.Visible() != 30 {
		t.Errorf("Visible() = %d, want 30", got.Visible())
	}

	s.Apply(batch(ev("win_close", args(2))))
	if _, ok := s.Snapshot().Viewports[2]; ok {
		t.Error("viewport survived win_close")
	}
}

func TestViewportVisibleClampsBotlinePastEnd(t *testing.T) {
	// At the end of the buffer Nvim reports botline one past line_count.
	v := uistate.Viewport{Topline: 270, Botline: 301, LineCount: 300}
	if v.Visible() != 30 {
		t.Errorf("Visible() = %d, want 30", v.Visible())
	}
}

// scrollSnap places windows and gives each a viewport, as Nvim would after
// a :vsplit of a 300-line file next to a short one, plus a float.
func scrollSnap() uistate.Snapshot {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(4, 10, 5)),
		ev("win_pos", args(2, rpcWindow(1000), 0, 0, 40, 20)),
		ev("win_pos", args(3, rpcWindow(1001), 0, 41, 39, 20)),
		ev("win_float_pos", args(4, rpcWindow(1002), "NW", 1, 1, 1, true)),
		ev("win_viewport", args(2, rpcWindow(1000), 0, 20, 0, 0, 300, 0)),
		ev("win_viewport", args(3, rpcWindow(1001), 0, 6, 0, 0, 5, 0)),
		ev("win_viewport", args(4, rpcWindow(1002), 0, 5, 0, 0, 500, 0)),
	))
	return s.Snapshot()
}

func TestScrollbarsOnlyForOverflowingSplits(t *testing.T) {
	m := render.Metrics{CellWidth: 9, CellHeight: 20}
	bars := render.Scrollbars(scrollSnap(), m)
	if len(bars) != 1 {
		t.Fatalf("got %d bars, want 1 (the float and the short buffer get none): %+v", len(bars), bars)
	}
	b := bars[0]
	if b.GridID != 2 || b.Win != 1000 {
		t.Fatalf("bar for grid %d / win %d, want grid 2 / win 1000", b.GridID, b.Win)
	}
	// Right edge of the left split, full window height.
	if b.Track.Max.X != 40*9 || b.Track.Min.Y != 0 || b.Track.Max.Y != 20*20 {
		t.Errorf("track = %v, want right edge x=%d, y 0..%d", b.Track, 40*9, 20*20)
	}
	if b.Track.Dx() < 4 || b.Track.Dx() > 9 {
		t.Errorf("track width = %d, want within one cell", b.Track.Dx())
	}
	// 20 of 300 lines on screen, at the top: 400px * 20/300.
	if b.Thumb.Min.Y != 0 || b.Thumb.Dy() != 26 {
		t.Errorf("thumb = %v, want top 0 and 26px tall", b.Thumb)
	}
}

func TestScrollbarThumbNeverShorterThanACell(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("win_pos", args(2, rpcWindow(1000), 0, 0, 40, 20)),
		ev("win_viewport", args(2, rpcWindow(1000), 0, 20, 0, 0, 100000, 0)),
	))
	bars := render.Scrollbars(s.Snapshot(), render.Metrics{CellWidth: 9, CellHeight: 20})
	if len(bars) != 1 || bars[0].Thumb.Dy() != 20 {
		t.Fatalf("bars = %+v, want one with a 20px (one cell) thumb", bars)
	}
}

func TestScrollbarsSplitSideBySideEachGetABar(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("win_pos", args(2, rpcWindow(1000), 0, 0, 40, 20)),
		ev("win_pos", args(3, rpcWindow(1001), 0, 41, 39, 20)),
		ev("win_viewport", args(2, rpcWindow(1000), 0, 20, 0, 0, 300, 0)),
		ev("win_viewport", args(3, rpcWindow(1001), 280, 301, 0, 0, 300, 0)),
	))
	bars := render.Scrollbars(s.Snapshot(), render.Metrics{CellWidth: 9, CellHeight: 20})
	if len(bars) != 2 {
		t.Fatalf("got %d bars, want one per split", len(bars))
	}
	for _, b := range bars {
		if b.GridID == 3 {
			if b.Track.Max.X != 80*9 {
				t.Errorf("right split bar ends at x=%d, want %d", b.Track.Max.X, 80*9)
			}
			if b.Thumb.Max.Y != b.Track.Max.Y {
				t.Errorf("thumb %v should touch the track bottom %d at the end of the buffer", b.Thumb, b.Track.Max.Y)
			}
		}
	}
}

func TestScrollbarToplineAtInvertsThumbPosition(t *testing.T) {
	b := render.Scrollbar{
		Track:     image.Rect(0, 100, 6, 500),
		Thumb:     image.Rect(0, 100, 6, 140),
		LineCount: 1000,
		Visible:   40,
	}
	travel := b.Track.Dy() - b.Thumb.Dy()
	cases := []struct {
		y, want int
	}{
		{b.Track.Min.Y - 50, 0},
		{b.Track.Min.Y, 0},
		{b.Track.Min.Y + travel/2, 480},
		{b.Track.Min.Y + travel, 1000},
		{b.Track.Max.Y + 50, 1000},
	}
	for _, tc := range cases {
		if got := b.ToplineAt(tc.y); got != tc.want {
			t.Errorf("ToplineAt(%d) = %d, want %d", tc.y, got, tc.want)
		}
	}
}
