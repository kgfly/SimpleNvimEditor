package nvimproc

import "sync"

// redrawQueue is an unbounded FIFO of pending `redraw` batches.
//
// Nvim's UI protocol is incremental: a batch says "row 7 changed to this",
// not "here is the whole screen". Nvim tracks what it has already sent and
// never repeats itself, so a batch that the client discards is screen
// content that is gone permanently -- the display stays wrong until
// something unrelated happens to redraw that exact region.
//
// That makes dropping batches under load the wrong trade, even though the
// alternative (blocking) would stall the msgpack-rpc read loop. A queue
// that grows instead of dropping avoids both: the reader never blocks, and
// no state is lost. Growth is naturally bounded, because it only occurs
// while the UI goroutine is briefly behind, and a burst is thousands of
// small slices at most.
//
// The failure this replaces was very visible: maximizing the window makes
// Nvim redraw every row at the new size, which is exactly when a fixed
// buffer overflows. The dropped rows left a black screen showing only the
// tabline and a stale cursor.
type redrawQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  [][][]interface{}
	closed bool
}

func newRedrawQueue() *redrawQueue {
	q := &redrawQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// push appends a batch. It never blocks and never discards.
func (q *redrawQueue) push(batch [][]interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.items = append(q.items, batch)
	q.cond.Signal()
}

// pop returns the oldest batch, waiting if the queue is empty. It reports
// false once the queue is closed and drained, which ends the consumer loop.
func (q *redrawQueue) pop() ([][]interface{}, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.items) == 0 {
		return nil, false
	}
	batch := q.items[0]
	// Release the reference so a long-lived queue doesn't pin batches
	// that have already been applied.
	q.items[0] = nil
	q.items = q.items[1:]
	return batch, true
}

// close wakes every waiting consumer so they can finish.
func (q *redrawQueue) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

// len reports the number of queued batches, for tests and diagnostics.
func (q *redrawQueue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
