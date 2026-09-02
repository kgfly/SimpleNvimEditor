package uistate_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kgfly/SimpleNvimEditor/internal/input"
	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// TestChineseTextRoundTripsThroughNvim is the end-to-end ground truth: type
// Chinese through the exact path the IME uses (input.EncodeText ->
// proc.Input) and read back what lands in the rendered grid.
//
// With ext_multigrid the buffer lives on grid 2+, NOT grid 1 (which only
// carries chrome such as the statusline), so the buffer grid is inspected
// directly rather than "the first non-empty row anywhere".
func TestChineseTextRoundTripsThroughNvim(t *testing.T) {
	const cols, rows = 80, 24
	proc, err := nvimproc.Spawn("nvim", []string{"--clean"}, nil, cols, rows)
	if err != nil {
		t.Skipf("nvim unavailable: %v", err)
	}
	defer proc.RequestQuit()

	st := uistate.New()
	go func() {
		for batch := range proc.Redraw {
			st.Apply(batch)
		}
	}()

	const want = "中文字"
	proc.Input("i")
	proc.Input(input.EncodeText(want))

	deadline := time.Now().Add(5 * time.Second)
	var line string
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		line = bufferLine(st.Snapshot())
		if strings.Contains(line, want) {
			break
		}
	}

	dumpGrids(t, st.Snapshot())

	if !strings.Contains(line, want) {
		t.Errorf("buffer grid row 0 = %q, want it to contain %q", line, want)
	}
}

func gridIDs(snap uistate.Snapshot) []int {
	ids := make([]int, 0, len(snap.Grids))
	for id := range snap.Grids {
		ids = append(ids, id)
	}
	for i := range ids {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	return ids
}

// rowText concatenates a row's cell text.
func rowText(cells []uistate.Cell) string {
	var b strings.Builder
	for i := range cells {
		b.WriteString(cells[i].Text)
	}
	return b.String()
}

// bufferLine returns row 0 of the buffer grid (the lowest id above 1).
func bufferLine(snap uistate.Snapshot) string {
	for _, id := range gridIDs(snap) {
		if id == 1 {
			continue
		}
		gv := snap.Grids[id]
		if len(gv.Data) > 0 {
			return strings.TrimRight(rowText(gv.Data[0]), " ")
		}
	}
	return ""
}

// dumpGrids prints each grid's first content row cell-by-cell, revealing
// exactly how nvim encoded the double-width characters.
func dumpGrids(t *testing.T, snap uistate.Snapshot) {
	t.Helper()
	for _, id := range gridIDs(snap) {
		gv := snap.Grids[id]
		t.Logf("=== grid %d (%d rows x %d cols) ===", id, gv.Rows, gv.Cols)
		for r := range gv.Data {
			cells := gv.Data[r]
			if strings.TrimSpace(rowText(cells)) == "" {
				continue
			}
			t.Logf("  row %d = %q", r, strings.TrimRight(rowText(cells), " "))
			var parts []string
			for c := 0; c < len(cells) && c < 8; c++ {
				parts = append(parts, fmt.Sprintf("[%d]=%q", c, cells[c].Text))
			}
			t.Logf("  cells: %s", strings.Join(parts, " "))
			break
		}
	}
}
