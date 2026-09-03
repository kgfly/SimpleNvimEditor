package render

import (
	"testing"

	"gioui.org/font"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
)

// TestNoTofuAtAnyWeight is the regression test for characters rendering as
// empty boxes.
//
// It asserts rather than reports. The neighbouring coverage/provenance
// tests only t.Logf their findings, which is why the bold-weight case
// regressed unnoticed: a chain that covered every character at Normal lost
// 你 and 吗 at Bold, because most macOS CJK faces ship no bold and the
// shaper then reports the rune as missing.
//
// Both weights are checked because font_family may carry a weight suffix
// ("Hack Nerd Font Mono Bold"), which applies to every family in the chain.
func TestNoTofuAtAnyWeight(t *testing.T) {
	chain := withFallbacks("Hack Nerd Font Mono")

	// Simplified-only Han (你 吗 测 试) is the interesting set: these are
	// absent from programming fonts that still carry the shared Han
	// characters (好 中 文), which is what made one line of Chinese come
	// out half text and half tofu.
	chars := []string{"你", "吗", "测", "试", "好", "中", "文", "知", "道"}

	for _, w := range []struct {
		name string
		w    font.Weight
	}{{"Normal", font.Normal}, {"Bold", font.Bold}} {
		t.Run(w.name, func(t *testing.T) {
			for _, ch := range chars {
				if isTofu(t, chain, w.w, ch) {
					t.Errorf("%q renders as tofu at %s weight; the fallback chain needs a family that covers it at this weight", ch, w.name)
				}
			}
		})
	}
}

// TestScriptFallbacksExcludeProportionalCatchAll documents why Arial
// Unicode MS must not be reintroduced: it matches nearly every codepoint,
// so it wins the fallback race from proper CJK faces, but it is
// proportional rather than monospace and lacks a bold. That combination
// made "你知道吗" render with characters of visibly different sizes.
func TestScriptFallbacksExcludeProportionalCatchAll(t *testing.T) {
	for _, f := range config.ScriptFallbacks() {
		if f == "Arial Unicode MS" {
			t.Errorf("Arial Unicode MS is in the fallback chain; it is proportional and boldless, which causes mismatched glyph sizes within a line")
		}
	}
}
