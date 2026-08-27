package render

import (
	"image"
	"testing"

	"gioui.org/op"
)

// benchFrame renders a full screen of text at the given grid size and
// density, which is what the editor does on every frame.
func benchFrame(b *testing.B, cols, rows int, px image.Point, scale float32) {
	fonts := testFonts(b, scale, px)
	snap := gridSnapshot(cols, rows)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var ops op.Ops
		Frame(newContext(&ops, px, scale), fonts, snap)
	}
}

// The two benchmarks below are the measurement that located the resize
// stall, and exist to keep it from creeping back. Both grid sizes were
// taken from a live run across two real monitors.
//
// The external panel is the slower case despite being physically smaller:
// at 1x its cells are 7x15 rather than 15x29, so the same screen holds
// roughly twice as many of them (491x91 ~= 45k cells vs 274x83 ~= 23k).

func BenchmarkFrameRetina2x(b *testing.B) {
	benchFrame(b, 274, 83, image.Pt(4112, 2412), 2)
}

func BenchmarkFrameExternal1x(b *testing.B) {
	benchFrame(b, 491, 91, image.Pt(3440, 1378), 1)
}
