package render

import (
	"bytes"
	"image"
	"testing"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
)

// facePixels renders one rune with an explicit typeface+weight.
func facePixels(t *testing.T, sh *text.Shaper, typeface string, w font.Weight, ch string) *image.RGBA {
	t.Helper()
	px := image.Pt(80, 50)
	f := Fonts{Shaper: sh, Face: font.Font{Typeface: font.Typeface(typeface), Weight: w}, Size: 12}
	var ops op.Ops
	f.Metrics = Measure(newContext(&ops, px, 2), f.Shaper, f.Face, unit.Sp(12))
	return rasterize(t, px, func(gtx layout.Context) { drawOne(gtx, f, ch) })
}

// TestFallbackChainIsConsistentAtBold is the regression test for mixed
// glyph sizes within a single line of Chinese.
//
// A fallback list must not contain a family that lacks the requested
// weight. When "Hack Nerd Font Mono Bold" was configured, the chain was
// requested at Weight Bold; Arial Unicode MS matched several CJK runes but
// ships no bold face, so the shaper substituted a differently-proportioned
// regular one and "你知道吗" came out with two characters visibly heavier
// and larger than the others.
//
// The invariant: at Bold, every character of a homogeneous CJK string must
// resolve to the SAME family it resolves to at Normal.
func TestFallbackChainIsConsistentAtBold(t *testing.T) {
	sh := NewShaper()
	chain := withFallbacks("Hack Nerd Font Mono")
	t.Logf("chain = %s", chain)

	// Candidate families the chain could resolve to.
	candidates := []string{"Hack Nerd Font Mono", "Menlo", "PingFang SC", "PingFang TC", "Hiragino Sans", "Apple SD Gothic Neo"}

	sourceOf := func(w font.Weight, ch string) string {
		got := facePixels(t, sh, chain, w, ch)
		for _, fam := range candidates {
			if bytes.Equal(facePixels(t, sh, fam, w, ch).Pix, got.Pix) {
				return fam
			}
		}
		return "UNKNOWN"
	}

	for _, ch := range []string{"你", "知", "道", "吗", "好", "中", "文"} {
		atNormal := sourceOf(font.Normal, ch)
		atBold := sourceOf(font.Bold, ch)
		t.Logf("  %s  normal=%-20s bold=%s", ch, atNormal, atBold)
		if atBold != atNormal {
			t.Errorf("%s resolves to %q at Normal but %q at Bold: the chain contains a family without a bold face, which is what makes glyph sizes differ within one line", ch, atNormal, atBold)
		}
	}
}
