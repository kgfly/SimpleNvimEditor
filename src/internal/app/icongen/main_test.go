package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestIconsUpToDate re-renders a sample of the committed icons, covering
// one- and two-digit numbers and light and dark backgrounds, so a change to
// the drawing can't silently alter them; run `go generate` after one.
func TestIconsUpToDate(t *testing.T) {
	for _, color := range []string{"black", "white", "blue"} {
		base, err := readPNG(filepath.Join("..", "icons", "icon_bg_"+color+".png"))
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range []int{2, 5, 9, 10, 11, 42, 80, 99} {
			name := fmt.Sprintf("icon_bg_%s_%d.png", color, n)
			want, err := os.ReadFile(filepath.Join("..", "icons", name))
			if err != nil {
				t.Fatal(err)
			}
			var got bytes.Buffer
			if err := encodePNG(&got, renderIcon(base, n)); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got.Bytes(), want) {
				t.Errorf("%s differs from what icongen draws", name)
			}
		}
	}
}
