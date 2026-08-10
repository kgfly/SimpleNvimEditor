package unit_test

import (
	"sync"
	"testing"

	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

func TestApplyReturnsTrueOnlyWhenFlushPresent(t *testing.T) {
	s := uistate.New()

	if flushed := s.Apply(batch(ev("grid_resize", args(1, 5, 5)))); flushed {
		t.Fatalf("Apply returned true without a flush event")
	}
	if flushed := s.Apply(batch(ev("grid_resize", args(1, 5, 5)), ev("flush"))); !flushed {
		t.Fatalf("Apply returned false even though the batch contained flush")
	}
}

func TestApplyIgnoresUnknownEvents(t *testing.T) {
	s := uistate.New()
	// Should not panic and should not affect state we can observe.
	s.Apply(batch(ev("popupmenu_show", args("irrelevant")), ev("tabline_update", args())))
	if len(s.Snapshot().Grids) != 0 {
		t.Fatalf("unknown events unexpectedly created grid state")
	}
}

func TestApplyToleratesMalformedTuples(t *testing.T) {
	s := uistate.New()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Apply panicked on malformed input: %v", r)
		}
	}()

	malformed := [][]interface{}{
		{},                          // empty tuple
		{"grid_resize"},             // event name with no args at all
		{"grid_resize", args()},     // invocation with zero positional args
		{"grid_line", args(1)},      // too few args for grid_line
		{123},                       // non-string event name
		{"grid_scroll", args(1, 2)}, // too few args for grid_scroll
	}
	s.Apply(malformed)
}

func TestApplyBusyStartStop(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("busy_start")))
	if !s.Snapshot().Busy {
		t.Fatalf("Busy = false after busy_start")
	}
	s.Apply(batch(ev("busy_stop")))
	if s.Snapshot().Busy {
		t.Fatalf("Busy = true after busy_stop")
	}
}

// TestConcurrentApplyAndSnapshot exercises State's locking under -race: many
// goroutines mutate and read the same State concurrently, mirroring the
// real program's split between the nvimproc redraw-pump goroutine and the
// Gio frame-drawing goroutine.
func TestConcurrentApplyAndSnapshot(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("grid_resize", args(1, 10, 10))))

	var wg sync.WaitGroup
	const n = 50

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			s.Apply(batch(ev("grid_line", args(1, i%10, 0, []interface{}{
				[]interface{}{"x"},
			}))))
		}(i)
	}

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_ = s.Snapshot()
		}()
	}

	wg.Wait()
}
