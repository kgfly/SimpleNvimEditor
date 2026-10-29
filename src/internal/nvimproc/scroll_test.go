package nvimproc

import (
	"sync"
	"testing"
)

// TestScrollWindowCoalescesDrag checks that a burst of scrollbar drag
// positions for one window sends only the latest, while another window's
// request is kept.
func TestScrollWindowCoalescesDrag(t *testing.T) {
	var mu sync.Mutex
	var got [][2]int
	p := &Process{slow: newFuncQueue()}
	p.scrollFn = func(win, top int, _ bool) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, [2]int{win, top})
	}

	for top := 0; top < 50; top++ {
		p.ScrollWindow(1000, top, false)
	}
	p.ScrollWindow(1001, 7, false)
	drain(p)
	p.ScrollWindow(1000, 99, false)
	drain(p)

	want := [][2]int{{1000, 49}, {1001, 7}, {1000, 99}}
	if len(got) != len(want) {
		t.Fatalf("sent %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sent %v, want %v", got, want)
		}
	}
}
