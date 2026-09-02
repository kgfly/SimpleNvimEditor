package render

import (
	"bytes"
	"image"
	"testing"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// isTofu detects .notdef by comparing against U+10FFFF, which is
// guaranteed absent from every font, rendered in the SAME family+weight.
func isTofu(t *testing.T, typeface string, w font.Weight, ch string) bool {
	t.Helper()
	px := image.Pt(80, 50)
	shot := func(s string) *image.RGBA {
		f := Fonts{Shaper: NewShaper(), Face: font.Font{Typeface: font.Typeface(typeface), Weight: w}, Size: 12}
		var ops op.Ops
		f.Metrics = Measure(newContext(&ops, px, 2), f.Shaper, f.Face, unit.Sp(12))
		return rasterize(t, px, func(gtx layout.Context) { drawOne(gtx, f, s) })
	}
	return bytes.Equal(shot(ch).Pix, shot("\U0010FFFF").Pix)
}

// TestPerFamilyCJKCoverageByWeight reports, for each candidate fallback
// family, whether it can draw each character at Normal and at Bold. This
// is the data needed to choose a chain that works at both weights.
func TestPerFamilyCJKCoverageByWeight(t *testing.T) {
	families := []string{
		"Hack Nerd Font Mono", "Menlo", "PingFang SC", "PingFang TC",
		"Hiragino Sans", "Apple SD Gothic Neo", "Arial Unicode MS",
		"Heiti SC", "STHeiti", "Songti SC", "Apple LiGothic",
	}
	chars := []string{"你", "吗", "知", "好"}

	for _, fam := range families {
		var normal, bold []string
		for _, ch := range chars {
			if isTofu(t, fam, font.Normal, ch) {
				normal = append(normal, ch+":TOFU")
			} else {
				normal = append(normal, ch+":ok")
			}
			if isTofu(t, fam, font.Bold, ch) {
				bold = append(bold, ch+":TOFU")
			} else {
				bold = append(bold, ch+":ok")
			}
		}
		t.Logf("%-22s normal=%v bold=%v", fam, normal, bold)
	}
}

// TestChainCoverageAtBold checks the real chain at Bold, the exact
// configuration the user runs.
func TestChainCoverageAtBold(t *testing.T) {
	chain := withFallbacks("Hack Nerd Font Mono")
	t.Logf("chain = %s", chain)
	for _, ch := range []string{"你", "吗", "知", "道", "好", "中", "文"} {
		t.Logf("  %s normal_tofu=%v bold_tofu=%v", ch,
			isTofu(t, chain, font.Normal, ch), isTofu(t, chain, font.Bold, ch))
	}
}
