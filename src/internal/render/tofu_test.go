package render

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
)

func fontsFor(t testing.TB, ed config.EditorConfig, scale float32, px image.Point) Fonts {
	t.Helper()
	f := Fonts{Shaper: NewShaper(), Face: FontFace(ed), Size: unit.Sp(ed.FontSize)}
	var ops op.Ops
	f.Metrics = Measure(newContext(&ops, px, scale), f.Shaper, f.Face, f.Size)
	return f
}

// drawOne paints one string in white on black through the real glyph-cache
// path used for every grid cell.
func drawOne(gtx layout.Context, fonts Fonts, s string) {
	sz := gtx.Constraints.Max
	paint.FillShape(gtx.Ops, color.NRGBA{A: 0xff}, clip.Rect(image.Rect(0, 0, sz.X, sz.Y)).Op())
	cache := newGlyphCache(gtx, fonts)
	drawText(gtx, cache, 0, 0, s, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
}

func inkCount(img *image.RGBA) int {
	n := 0
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 0 || img.Pix[i+1] != 0 || img.Pix[i+2] != 0 {
			n++
		}
	}
	return n
}

// TestCJKIsNotTofu distinguishes real Chinese glyphs from .notdef boxes.
//
// Counting ink is not enough: a tofu box is ink too, and is exactly what a
// font lacking CJK coverage draws. The discriminator is that every missing
// codepoint maps to the SAME .notdef glyph, so distinct characters
// rasterize to byte-identical images. Real glyphs never do.
func TestCJKIsNotTofu(t *testing.T) {
	px := image.Pt(120, 60)

	cases := []struct {
		name   string
		editor config.EditorConfig
	}{
		{"default-bundled-gomono", config.Default().Editor},
		{"brownie-hack-nerd-bold", config.EditorConfig{FontSize: 12, UseSystemFonts: true, FontFamily: "Hack Nerd Font Mono Bold"}},
		{"menlo", config.EditorConfig{FontSize: 12, UseSystemFonts: true, FontFamily: "Menlo"}},
		{"pingfang-sc", config.EditorConfig{FontSize: 12, UseSystemFonts: true, FontFamily: "PingFang SC"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fonts := fontsFor(t, tc.editor, 2, px)
			shot := func(s string) *image.RGBA {
				return rasterize(t, px, func(gtx layout.Context) { drawOne(gtx, fonts, s) })
			}

			zhong, wen, zi := shot("\u4e2d"), shot("\u6587"), shot("\u5b57")
			a, b := shot("A"), shot("B")

			t.Logf("ink: zhong=%d wen=%d zi=%d | A=%d B=%d",
				inkCount(zhong), inkCount(wen), inkCount(zi), inkCount(a), inkCount(b))

			if bytes.Equal(a.Pix, b.Pix) {
				t.Fatalf("latin A and B rasterized identically; probe is broken")
			}

			sameZW := bytes.Equal(zhong.Pix, wen.Pix)
			sameWZ := bytes.Equal(wen.Pix, zi.Pix)
			t.Logf("identical? zhong==wen:%v wen==zi:%v", sameZW, sameWZ)

			if sameZW && sameWZ {
				t.Errorf("TOFU: three different Chinese characters drew identical pixels -> font has no CJK coverage, shaper is drawing .notdef")
			}
		})
	}
}
