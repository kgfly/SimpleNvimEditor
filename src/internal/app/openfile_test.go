package editorapp

import (
	"fmt"
	"sync"
	"testing"
)

// resetPendingOpens clears global queue state so tests don't leak into
// each other.
func resetPendingOpens(t *testing.T) {
	t.Helper()
	takeQueuedOpens()
	t.Cleanup(func() { takeQueuedOpens() })
}

func TestQueueOpenFileRoundTrips(t *testing.T) {
	resetPendingOpens(t)

	queueOpenFile("/tmp/a.txt")
	queueOpenFile("/tmp/b.txt")

	got := takeQueuedOpens()
	want := []string{"/tmp/a.txt", "/tmp/b.txt"}
	if len(got) != len(want) {
		t.Fatalf("takeQueuedOpens() = %v, want %v", got, want)
	}
	// Order matters: opening several files should edit them in the order
	// the desktop environment listed them.
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("path %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestQueueOpenFileIgnoresEmpty guards the queue against a path the
// platform failed to decode: an empty string would become a bare ":edit".
func TestQueueOpenFileIgnoresEmpty(t *testing.T) {
	resetPendingOpens(t)

	queueOpenFile("")
	if got := takeQueuedOpens(); got != nil {
		t.Errorf("takeQueuedOpens() = %v, want nil", got)
	}
}

// TestTakeQueuedOpensDrains verifies the queue is emptied by a read, so a
// file is opened exactly once rather than on every subsequent frame.
func TestTakeQueuedOpensDrains(t *testing.T) {
	resetPendingOpens(t)

	queueOpenFile("/tmp/a.txt")
	if got := takeQueuedOpens(); len(got) != 1 {
		t.Fatalf("first take = %v, want 1 path", got)
	}
	if got := takeQueuedOpens(); got != nil {
		t.Errorf("second take = %v, want nil (queue should be drained)", got)
	}
}

// TestQueueOpenFileIsThreadSafe exercises the reason the queue exists.
//
// The platform delivers paths on its own thread (the AppKit main thread on
// macOS) while the editor drains them from Gio's event loop. Run with
// -race, this asserts those two sides cannot corrupt the slice.
func TestQueueOpenFileIsThreadSafe(t *testing.T) {
	resetPendingOpens(t)

	const writers, perWriter = 8, 50

	var wg sync.WaitGroup
	wg.Add(writers)
	for w := 0; w < writers; w++ {
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				queueOpenFile(fmt.Sprintf("/tmp/%d-%d.txt", w, i))
			}
		}(w)
	}

	// Drain concurrently with the writers, collecting as we go.
	done := make(chan int)
	go func() {
		seen := 0
		for {
			seen += len(takeQueuedOpens())
			select {
			case <-done:
				done <- seen + len(takeQueuedOpens())
				return
			default:
			}
		}
	}()

	wg.Wait()
	done <- 0
	total := <-done

	if want := writers * perWriter; total != want {
		t.Errorf("collected %d paths, want %d (none may be lost or duplicated)", total, want)
	}
}

// TestDrainOpenRequestsWithoutNvimKeepsPaths is the regression test for
// opening an empty editor.
//
// On macOS the Apple Event naming the file arrives during launch, before
// Nvim has been spawned. If drain discarded the queue while a.proc was
// nil, the file that caused the launch would be silently dropped -- the
// app would come up blank.
func TestDrainOpenRequestsWithoutNvimKeepsPaths(t *testing.T) {
	resetPendingOpens(t)

	queueOpenFile("/tmp/launch.txt")

	a := &App{} // proc is nil: Nvim has not started yet
	a.drainOpenRequests()

	got := takeQueuedOpens()
	if len(got) != 1 || got[0] != "/tmp/launch.txt" {
		t.Errorf("after draining with no Nvim, queue = %v, want the path still pending", got)
	}
}
