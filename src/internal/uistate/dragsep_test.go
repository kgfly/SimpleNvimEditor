package uistate_test

import (
	"sort"
	"testing"
	"time"

	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// TestSeparatorDragLandsUnderPointer drags the separator between a diff
// window and a fixed-width side panel across the diff window, the way a
// user resizes gh.nvim's PR panel, and checks it ends at the release column.
func TestSeparatorDragLandsUnderPointer(t *testing.T) {
	const cols, rows = 160, 20
	layout := "vsplit | vsplit | wincmd l | wincmd l | setl winfixwidth | vertical resize 40 | wincmd h | diffthis | wincmd h | diffthis"
	proc, err := nvimproc.Spawn("nvim", []string{"--clean", "-c", "set mouse=a", "-c", layout}, nil, cols, rows)
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

	var wins []uistate.Placement
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && len(wins) < 3 {
		time.Sleep(100 * time.Millisecond)
		wins = nil
		for _, p := range st.Snapshot().Windows {
			if p.GridID != 1 && !p.Float {
				wins = append(wins, p)
			}
		}
	}
	if len(wins) < 3 {
		t.Fatalf("expected three split windows, got %+v", wins)
	}
	sort.Slice(wins, func(i, j int) bool { return wins[i].Col < wins[j].Col })

	mid := wins[1]
	sep := mid.Col + mid.Width
	const row = 5
	target := sep - 30

	// Mirrors App.onPointer: hit-test the press, pin drags/release to its grid.
	pressGrid := 0
	send := func(action string, col int) {
		snap := st.Snapshot()
		grid, gr, gc, ok := uistate.HitTest(snap.Windows, row, col)
		if !ok {
			grid, gr, gc = 1, row, col
		}
		if action == "press" {
			pressGrid = grid
		} else {
			grid, gr, gc = uistate.DragTarget(snap.Windows, pressGrid, row, col)
		}
		proc.InputMouse("left", action, "", grid, gr, gc)
		time.Sleep(30 * time.Millisecond)
	}
	send("press", sep)
	for c := sep - 3; c >= target; c -= 3 {
		send("drag", c)
	}
	send("release", target)

	want := target - mid.Col
	var got int
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		for _, p := range st.Snapshot().Windows {
			if p.GridID == mid.GridID {
				got = p.Width
			}
		}
		if got == want {
			return
		}
	}
	t.Fatalf("middle window width = %d, want %d (separator should end at col %d)", got, want, target)
}
