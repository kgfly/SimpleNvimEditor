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

func TestApplyGridClear(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(1, 5, 3)),
		ev("grid_line", args(1, 0, 0, []interface{}{[]interface{}{"hello"}})),
	))
	snap := s.Snapshot()
	if snap.Grids[1].Data[0][0].Text != "hello" {
		t.Fatalf("setup: expected 'hello' in cell")
	}

	s.Apply(batch(ev("grid_clear", args(1))))
	snap = s.Snapshot()
	if snap.Grids[1].Data[0][0].Text != " " {
		t.Fatalf("after grid_clear: cell = %q, want space", snap.Grids[1].Data[0][0].Text)
	}
}

func TestApplySetTitle(t *testing.T) {
	s := uistate.New()
	// set_title sends [["path/to/file.txt"]]
	s.Apply(batch(ev("set_title", args("file.txt"))))
	if got := s.Snapshot().Title; got != "file.txt" {
		t.Fatalf("Title = %q, want %q", got, "file.txt")
	}
}

func TestApplySetTitleStripsPrefix(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("set_title", args("Dir:sub file.go"))))
	if got := s.Snapshot().Title; got != "file.go" {
		t.Fatalf("Title = %q, want %q", got, "file.go")
	}
}

func TestApplySetTitleEmpty(t *testing.T) {
	s := uistate.New()
	// Empty args should not panic.
	s.Apply(batch(ev("set_title")))
	if got := s.Snapshot().Title; got != "" {
		t.Fatalf("Title = %q, want empty", got)
	}
}

func TestApplySetTitleEmptyRow(t *testing.T) {
	s := uistate.New()
	// An invocation with an empty row slice.
	s.Apply(batch(ev("set_title", []interface{}{})))
	if got := s.Snapshot().Title; got != "" {
		t.Fatalf("Title = %q, want empty", got)
	}
}

func TestApplySetTitleNoSpaceAfterColon(t *testing.T) {
	s := uistate.New()
	// A title with a colon but no space after it should remain as-is.
	s.Apply(batch(ev("set_title", args("foo:bar"))))
	if got := s.Snapshot().Title; got != "foo:bar" {
		t.Fatalf("Title = %q, want %q", got, "foo:bar")
	}
}

func TestApplySetTitleNonStringRow(t *testing.T) {
	s := uistate.New()
	// row[0] is not a string — should not panic.
	s.Apply(batch(ev("set_title", args(12345))))
	if got := s.Snapshot().Title; got != "" {
		t.Fatalf("Title = %q, want empty", got)
	}
}

func TestToIntVariousTypes(t *testing.T) {
	s := uistate.New()
	// Exercise the int32, uint32, and float64 branches of toInt by passing
	// those types into grid_resize (which calls toInt on each arg).
	s.Apply(batch(ev("grid_resize", args(int32(1), uint32(8), float64(4)))))
	snap := s.Snapshot()
	g, ok := snap.Grids[1]
	if !ok {
		t.Fatalf("grid 1 not created")
	}
	if g.Cols != 8 || g.Rows != 4 {
		t.Fatalf("grid size = %dx%d, want 8x4", g.Cols, g.Rows)
	}
}

func TestGridDestroy(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(2, 10, 5)),
		ev("win_pos", args(2, 0, 0, 0, 10, 5)),
	))
	if _, ok := s.Snapshot().Grids[2]; !ok {
		t.Fatalf("setup: grid 2 not created")
	}
	s.Apply(batch(ev("grid_destroy", args(2))))
	if _, ok := s.Snapshot().Grids[2]; ok {
		t.Fatalf("grid 2 still present after grid_destroy")
	}
	if len(s.Snapshot().Windows) != 0 {
		t.Fatalf("window placement still present after grid_destroy")
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
