package render

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"

	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

var (
	cRed   = color.NRGBA{R: 0xff, A: 0xff}
	cBlack = color.NRGBA{A: 0xff}
	cWhite = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	cBlue  = color.NRGBA{B: 0xff, A: 0xff}
)

// cursorState builds a 1-row grid "aaa" with the cursor on the middle cell,
// default colors white-on-black, and the given rgb_attr as hl id 7 used by
// the (only) normal-mode cursor.
func cursorState(shape string, attrID int, rgbAttr map[string]interface{}) uistate.Snapshot {
	s := uistate.New()
	s.Apply([][]interface{}{
		{"default_colors_set", []interface{}{int64(0xffffff), int64(0x000000), int64(0), int64(0), int64(0)}},
		{"hl_attr_define", []interface{}{int64(7), rgbAttr, map[string]interface{}{}, []interface{}{}}},
		{"grid_resize", []interface{}{1, 3, 1}},
		{"grid_line", []interface{}{1, 0, 0, []interface{}{[]interface{}{"a"}, []interface{}{"a"}, []interface{}{"a"}}}},
		{"grid_cursor_goto", []interface{}{1, 0, 1}},
		{"mode_info_set", []interface{}{true, []interface{}{
			map[string]interface{}{"cursor_shape": shape, "cell_percentage": 100, "attr_id": int64(attrID)},
		}}},
		{"mode_change", []interface{}{"normal", 0}},
	})
	return s.Snapshot()
}

func TestCursorColors(t *testing.T) {
	cases := []struct {
		name        string
		attrID      int
		rgbAttr     map[string]interface{}
		block, text color.NRGBA
	}{
		{"attr 0 swaps cell colors", 0, map[string]interface{}{}, cWhite, cBlack},
		{"fg and bg set", 7, map[string]interface{}{"foreground": int64(0x000000), "background": int64(0xff0000)}, cRed, cBlack},
		{"only bg set keeps text readable", 7, map[string]interface{}{"background": int64(0xff0000)}, cRed, cBlack},
		{"reverse", 7, map[string]interface{}{"reverse": true, "foreground": int64(0x0000ff)}, cBlue, cBlack},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := cursorState("block", tc.attrID, tc.rgbAttr)
			block, text := cursorColors(snap.Highlight, tc.attrID, 0)
			if block != tc.block || text != tc.text {
				t.Errorf("got block=%v text=%v, want block=%v text=%v", block, text, tc.block, tc.text)
			}
		})
	}
}

func TestCursorCellDoubleWidth(t *testing.T) {
	snap := uistate.Snapshot{
		Grids: map[int]uistate.GridView{1: {ID: 1, Rows: 1, Cols: 3, Data: [][]uistate.Cell{
			{{Text: "W"}, {Text: ""}, {Text: "x"}},
		}}},
		Cursor: uistate.Cursor{GridID: 1, Col: 0},
	}
	if cell, w := cursorCell(snap); cell.Text != "W" || w != 2 {
		t.Errorf("wide cell: got %q width %d, want \"W\" width 2", cell.Text, w)
	}
	snap.Cursor.Col = 2
	if cell, w := cursorCell(snap); cell.Text != "x" || w != 1 {
		t.Errorf("narrow cell: got %q width %d, want \"x\" width 1", cell.Text, w)
	}
	snap.Cursor.Col = 9
	if _, w := cursorCell(snap); w != 1 {
		t.Errorf("out of range: got width %d, want 1", w)
	}
}

// countColor counts pixels inside r that exactly match c.
func countColor(img *image.RGBA, r image.Rectangle, c color.NRGBA) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			p := img.RGBAAt(x, y)
			if p.R == c.R && p.G == c.G && p.B == c.B {
				n++
			}
		}
	}
	return n
}

// TestBlockCursorShowsCharacter checks the cursor cell is filled with the
// cursor color but the character under it is still drawn (in the text
// color), rather than being hidden by an opaque block.
func TestBlockCursorShowsCharacter(t *testing.T) {
	px := image.Pt(120, 60)
	fonts := testFonts(t, 1, px)
	cw, ch := fonts.Metrics.CellWidth, fonts.Metrics.CellHeight
	cellRect := image.Rect(cw, 0, 2*cw, ch)

	snap := cursorState("block", 7, map[string]interface{}{"foreground": int64(0x000000), "background": int64(0xff0000)})
	img := rasterize(t, px, func(gtx layout.Context) { Frame(gtx, fonts, snap) })

	total := cw * ch
	red := countColor(img, cellRect, cRed)
	if red == 0 {
		t.Fatal("cursor cell has no cursor-colored pixels")
	}
	if red == total {
		t.Error("cursor cell is a solid block; the character under it was not drawn")
	}
}

// TestBlendedCursorIsHidden covers smear-cursor style hiding: a cursor whose
// highlight has blend=100 must not be painted.
func TestBlendedCursorIsHidden(t *testing.T) {
	px := image.Pt(120, 60)
	fonts := testFonts(t, 1, px)
	cw, ch := fonts.Metrics.CellWidth, fonts.Metrics.CellHeight
	cellRect := image.Rect(cw, 0, 2*cw, ch)

	snap := cursorState("block", 7, map[string]interface{}{"background": int64(0xff0000), "blend": int64(100)})
	img := rasterize(t, px, func(gtx layout.Context) { Frame(gtx, fonts, snap) })

	if red := countColor(img, cellRect, cRed); red != 0 {
		t.Errorf("blend=100 cursor painted %d red pixels, want 0", red)
	}
}

// TestMessageGridClippedToBaseGrid: Nvim sizes the message grid to the full
// screen and parks it at the cmdline row, so its stale lower lines must not
// leak into the leftover pixels below the base grid.
func TestMessageGridClippedToBaseGrid(t *testing.T) {
	fonts := testFonts(t, 1, image.Pt(120, 120))
	cw, ch := fonts.Metrics.CellWidth, fonts.Metrics.CellHeight
	const cols, rows = 4, 3
	px := image.Pt(cols*cw, rows*ch+ch/2)

	red := []interface{}{"x", int64(7), int64(cols)}
	s := uistate.New()
	s.Apply([][]interface{}{
		{"default_colors_set", []interface{}{int64(0xffffff), int64(0x000000), int64(0), int64(0), int64(0)}},
		{"hl_attr_define", []interface{}{int64(7), map[string]interface{}{"background": int64(0xff0000)}, map[string]interface{}{}, []interface{}{}}},
		{"grid_resize", []interface{}{int64(1), int64(cols), int64(rows)}},
		{"grid_resize", []interface{}{int64(3), int64(cols), int64(rows)}},
		{"grid_line", []interface{}{int64(3), int64(1), int64(0), []interface{}{red}}},
		{"msg_set_pos", []interface{}{int64(3), int64(rows - 1), false, ""}},
	})
	snap := s.Snapshot()
	img := rasterize(t, px, func(gtx layout.Context) { Frame(gtx, fonts, snap) })

	if n := countColor(img, image.Rect(0, rows*ch, px.X, px.Y), cRed); n != 0 {
		t.Errorf("message grid painted %d pixels below the base grid", n)
	}
}
