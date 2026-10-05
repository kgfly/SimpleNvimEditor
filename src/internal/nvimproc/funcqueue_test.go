package nvimproc

import (
	"testing"
	"time"
)

// TestInputNeverBlocksWhenNvimIsStuck is the regression test for the window
// freezing (Cmd+Q included) once Nvim stops answering. Input runs on Gio's
// UI goroutine; with a bounded queue it blocked as soon as the backlog of
// keystrokes and mouse events filled the buffer.
func TestInputNeverBlocksWhenNvimIsStuck(t *testing.T) {
	p := &Process{cmds: newFuncQueue()} // nothing consumes: Nvim is "hung"

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100000; i++ {
			p.InputMouse("", "move", "", 1, i%50, i%80)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("InputMouse blocked the caller while Nvim was not draining the queue")
	}
	if n := p.cmds.len(); n != 100000 {
		t.Errorf("queued %d calls, want 100000 (none may be dropped)", n)
	}
}

func TestFuncQueuePreservesOrder(t *testing.T) {
	q := newFuncQueue()
	var got []int
	for i := 0; i < 5; i++ {
		i := i
		q.push(func() { got = append(got, i) })
	}
	for {
		fn, ok := q.tryPop()
		if !ok {
			break
		}
		fn()
	}
	for i, v := range got {
		if v != i {
			t.Fatalf("ran %v, want 0..4 in order", got)
		}
	}
	if len(got) != 5 {
		t.Fatalf("ran %d calls, want 5", len(got))
	}
}

// TestFuncQueueCloseReleasesWorker ensures the worker goroutine ends when
// Nvim exits instead of leaking, blocked in pop.
func TestFuncQueueCloseReleasesWorker(t *testing.T) {
	q := newFuncQueue()
	done := make(chan struct{})
	go func() {
		run(q)
		close(done)
	}()
	q.close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker still blocked after close")
	}
	q.push(func() { t.Error("call ran after close") })
	if _, ok := q.tryPop(); ok {
		t.Error("closed queue still yields calls")
	}
}
