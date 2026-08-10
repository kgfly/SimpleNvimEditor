package unit_test

import (
	"testing"

	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

func TestGridResizeCreatesBlankGrid(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("grid_resize", args(1, 10, 4)))) // [grid, cols, rows]

	snap := s.Snapshot()
	g, ok := snap.Grids[1]
	if !ok {
		t.Fatalf("expected grid 1 to exist after grid_resize")
	}
	if g.Cols != 10 || g.Rows != 4 {
		t.Fatalf("got cols=%d rows=%d, want cols=10 rows=4", g.Cols, g.Rows)
	}
	if len(g.Data) != 4 {
		t.Fatalf("got %d rows of data, want 4", len(g.Data))
	}
	for r, row := range g.Data {
		if len(row) != 10 {
			t.Fatalf("row %d has %d cells, want 10", r, len(row))
		}
		for c, cell := range row {
			if cell.Text != " " {
				t.Fatalf("cell [%d][%d] = %q, want blank space", r, c, cell.Text)
			}
		}
	}
}

func TestGridLineSimpleRun(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(1, 20, 3)),
		ev("grid_line", args(1, 0, 0, []interface{}{
			[]interface{}{"h"},
			[]interface{}{"i"},
		})),
	))

	g := s.Snapshot().Grids[1]
	if got := g.Data[0][0].Text; got != "h" {
		t.Fatalf("cell [0][0] = %q, want %q", got, "h")
	}
	if got := g.Data[0][1].Text; got != "i" {
		t.Fatalf("cell [0][1] = %q, want %q", got, "i")
	}
}

func TestGridLineRepeatAndHlCarryOver(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(1, 20, 1)),
		// [text, hl_id, repeat] then a [text]-only cell, which per the
		// protocol reuses the previous cell's highlight id (5 here).
		ev("grid_line", args(1, 0, 0, []interface{}{
			[]interface{}{"x", 5, 3},
			[]interface{}{"y"},
		})),
	))

	row := s.Snapshot().Grids[1].Data[0]
	for i := 0; i < 3; i++ {
		if row[i].Text != "x" || row[i].HlID != 5 {
			t.Fatalf("cell %d = %+v, want {x 5}", i, row[i])
		}
	}
	if row[3].Text != "y" || row[3].HlID != 5 {
		t.Fatalf("cell 3 = %+v, want {y 5} (hl carried over)", row[3])
	}
}

func TestGridLineOutOfBoundsRowIsIgnored(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(1, 5, 2)),
		ev("grid_line", args(1, 99, 0, []interface{}{
			[]interface{}{"z"},
		})),
	))
	// Should not panic, and shouldn't touch valid rows either.
	g := s.Snapshot().Grids[1]
	for r, row := range g.Data {
		for c, cell := range row {
			if cell.Text != " " {
				t.Fatalf("cell [%d][%d] = %q, want untouched blank", r, c, cell.Text)
			}
		}
	}
}

func TestGridClear(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(1, 5, 1)),
		ev("grid_line", args(1, 0, 0, []interface{}{[]interface{}{"a"}})),
	))
	if s.Snapshot().Grids[1].Data[0][0].Text != "a" {
		t.Fatalf("setup failed: expected 'a' before clear")
	}

	s.Apply(batch(ev("grid_clear", args(1))))
	for _, cell := range s.Snapshot().Grids[1].Data[0] {
		if cell.Text != " " {
			t.Fatalf("cell = %q after grid_clear, want blank", cell.Text)
		}
	}
}

func TestGridDestroyRemovesGridAndPlacement(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(2, 5, 1)),
		ev("win_pos", args(2, 0, 1, 3, 5, 1)),
	))
	if _, ok := s.Snapshot().Grids[2]; !ok {
		t.Fatalf("setup failed: expected grid 2 to exist")
	}

	s.Apply(batch(ev("grid_destroy", args(2))))

	snap := s.Snapshot()
	if _, ok := snap.Grids[2]; ok {
		t.Fatalf("grid 2 still present after grid_destroy")
	}
	for _, p := range snap.Windows {
		if p.GridID == 2 {
			t.Fatalf("placement for grid 2 still present after grid_destroy")
		}
	}
}

func TestGridScrollShiftsContentUp(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("grid_resize", args(1, 1, 4))))
	// Seed each row with a distinct marker so we can tell rows apart.
	for r := 0; r < 4; r++ {
		marker := string(rune('a' + r))
		s.Apply(batch(ev("grid_line", args(1, r, 0, []interface{}{[]interface{}{marker}}))))
	}

	// Scroll rows [0,4) by +1: content moves up, row 3 is exposed blank.
	// grid_scroll args: [grid, top, bot, left, right, rows, cols]
	s.Apply(batch(ev("grid_scroll", args(1, 0, 4, 0, 1, 1, 0))))

	g := s.Snapshot().Grids[1]
	want := []string{"b", "c", "d", " "}
	for r, w := range want {
		if got := g.Data[r][0].Text; got != w {
			t.Fatalf("row %d = %q, want %q", r, got, w)
		}
	}
}

func TestGridScrollShiftsContentDown(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("grid_resize", args(1, 1, 4))))
	for r := 0; r < 4; r++ {
		marker := string(rune('a' + r))
		s.Apply(batch(ev("grid_line", args(1, r, 0, []interface{}{[]interface{}{marker}}))))
	}

	// Negative rowsBy scrolls content down (rows param -1).
	s.Apply(batch(ev("grid_scroll", args(1, 0, 4, 0, 1, -1, 0))))

	g := s.Snapshot().Grids[1]
	want := []string{" ", "a", "b", "c"}
	for r, w := range want {
		if got := g.Data[r][0].Text; got != w {
			t.Fatalf("row %d = %q, want %q", r, got, w)
		}
	}
}

func TestGridScrollZeroDeltaIsNoop(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(
		ev("grid_resize", args(1, 1, 2)),
		ev("grid_line", args(1, 0, 0, []interface{}{[]interface{}{"a"}})),
		ev("grid_line", args(1, 1, 0, []interface{}{[]interface{}{"b"}})),
		ev("grid_scroll", args(1, 0, 2, 0, 1, 0, 0)),
	))
	g := s.Snapshot().Grids[1]
	if g.Data[0][0].Text != "a" || g.Data[1][0].Text != "b" {
		t.Fatalf("zero-delta scroll mutated content: %+v", g.Data)
	}
}
