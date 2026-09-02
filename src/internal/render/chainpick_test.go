package render

import (
	"testing"

	"gioui.org/font"
)

// TestCandidateChainsAtBothWeights evaluates chains for full coverage at
// BOTH Normal and Bold, since the user's font_family may carry a weight
// suffix that is applied to every family in the list.
func TestCandidateChainsAtBothWeights(t *testing.T) {
	chars := []string{"你", "吗", "知", "道", "好", "中", "文", "测", "试"}

	chains := map[string]string{
		"current":       `"Hack Nerd Font Mono", Menlo, "PingFang SC", "PingFang TC", "Hiragino Sans", "Apple SD Gothic Neo"`,
		"plus-heiti":    `"Hack Nerd Font Mono", Menlo, "PingFang SC", "Heiti SC", "Songti SC", "Hiragino Sans", "Apple SD Gothic Neo"`,
		"heiti-early":   `"Hack Nerd Font Mono", "Heiti SC", "PingFang SC", "Hiragino Sans", "Apple SD Gothic Neo"`,
		"songti-early":  `"Hack Nerd Font Mono", "Songti SC", "PingFang SC", "Hiragino Sans", "Apple SD Gothic Neo"`,
	}

	for name, chain := range chains {
		t.Run(name, func(t *testing.T) {
			bad := 0
			for _, ch := range chars {
				n := isTofu(t, chain, font.Normal, ch)
				b := isTofu(t, chain, font.Bold, ch)
				if n || b {
					bad++
					t.Logf("  %s normal_tofu=%v bold_tofu=%v", ch, n, b)
				}
			}
			if bad == 0 {
				t.Logf("  FULL COVERAGE at both weights")
			}
		})
	}
}
