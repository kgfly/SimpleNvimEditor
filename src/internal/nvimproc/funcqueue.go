package nvimproc

import "sync"

// funcQueue is an unbounded FIFO of outgoing calls.
//
// Its producers run on Gio's UI goroutine, which must never block: if it
// does, the window stops repainting and even Cmd+Q / Alt+F4 go unanswered.
// A fixed-size channel blocks the sender once full, and it fills exactly
// when Nvim stops answering (a runaway Lua loop, a long synchronous
// system() call) while the user keeps typing, scrolling or moving the
// mouse. Growing instead keeps the UI alive so a force quit is still
// possible; see RequestQuit.
type funcQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []func()
	closed bool
}

func newFuncQueue() *funcQueue {
	q := &funcQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// push appends fn. It never blocks.
func (q *funcQueue) push(fn func()) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.items = append(q.items, fn)
	q.cond.Signal()
}

// pop returns the oldest call, waiting if the queue is empty. It reports
// false once the queue is closed, which ends the worker loop.
func (q *funcQueue) pop() (func(), bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	return q.take()
}

// tryPop is pop without waiting, for tests that drive the queue by hand.
func (q *funcQueue) tryPop() (func(), bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.take()
}

func (q *funcQueue) take() (func(), bool) {
	if q.closed || len(q.items) == 0 {
		return nil, false
	}
	fn := q.items[0]
	q.items[0] = nil
	q.items = q.items[1:]
	return fn, true
}

// close discards what is left (Nvim is gone, so every call would fail) and
// wakes the worker so it can return.
func (q *funcQueue) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.items = nil
	q.cond.Broadcast()
}

// len reports the number of queued calls, for tests.
func (q *funcQueue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
