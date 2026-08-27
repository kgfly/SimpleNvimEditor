package nvimproc

import (
	"sync"
	"testing"
)

// newResizeProbe returns a Process whose resize round trip is recorded
// instead of sent, plus an accessor for what it received. The command queue
// is drained manually so a test controls exactly when queued work runs,
// which is what makes the coalescing window observable.
func newResizeProbe() (*Process, func() [][2]int) {
	var mu sync.Mutex
	var got [][2]int

	p := &Process{cmds: make(chan func(), 1024)}
	p.resizeFn = func(cols, rows int) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, [2]int{cols, rows})
	}

	return p, func() [][2]int {
		mu.Lock()
		defer mu.Unlock()
		return append([][2]int(nil), got...)
	}
}

// drain runs every command currently queued, in order.
func drain(p *Process) {
	for {
		select {
		case fn := <-p.cmds:
			fn()
		default:
			return
		}
	}
}

// TestResizeCoalescesBurst is the regression test for the editor lagging
// behind the window edge while a resize drag is in progress.
//
// Dragging produces a new size every frame. Each TryResizeUI is a blocking
// round trip that makes Nvim reflow the whole screen, so forwarding all of
// them queues far more work than the gesture is worth -- and every result
// except the last is obsolete before it arrives.
func TestResizeCoalescesBurst(t *testing.T) {
	p, got := newResizeProbe()

	// A drag: fifty intermediate sizes, none of which get a chance to run
	// before the next arrives.
	for i := 0; i < 50; i++ {
		p.Resize(100+i, 30+i)
	}
	drain(p)

	sizes := got()
	if len(sizes) != 1 {
		t.Fatalf("a 50-frame drag produced %d resize calls (%v), want exactly 1", len(sizes), sizes)
	}
	if want := [2]int{149, 79}; sizes[0] != want {
		t.Errorf("coalesced to %v, want the final size %v", sizes[0], want)
	}
}

// TestResizeAlwaysDeliversFinalSize guards the property that actually
// matters: whatever the coalescing does, Nvim must end up agreeing with the
// window. Dropping the last size would leave the grid permanently
// mismatched -- a worse bug than the lag being fixed.
func TestResizeAlwaysDeliversFinalSize(t *testing.T) {
	p, got := newResizeProbe()

	p.Resize(80, 24)
	drain(p) // let the first one through, as a slow drag would

	p.Resize(120, 40)
	p.Resize(200, 60) // supersedes the pending one
	drain(p)

	sizes := got()
	if len(sizes) == 0 {
		t.Fatal("no resize reached Nvim")
	}
	if last, want := sizes[len(sizes)-1], [2]int{200, 60}; last != want {
		t.Errorf("last resize = %v, want %v; the grid would stay out of sync with the window", last, want)
	}
}

// TestResizeAfterDrainQueuesAgain ensures coalescing does not latch: once a
// queued resize has run, a later one must be sent rather than swallowed.
func TestResizeAfterDrainQueuesAgain(t *testing.T) {
	p, got := newResizeProbe()

	for _, sz := range [][2]int{{80, 24}, {90, 30}, {100, 36}} {
		p.Resize(sz[0], sz[1])
		drain(p)
	}

	sizes := got()
	if len(sizes) != 3 {
		t.Fatalf("three separate resizes produced %d calls (%v), want 3", len(sizes), sizes)
	}
}
