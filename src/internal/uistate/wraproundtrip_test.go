package uistate_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// TestSoftWrappedLineReportsWrapFlag checks that Nvim marks a soft-wrapped
// buffer row with grid_line's wrap flag, which link detection relies on to
// join a URL split across rows.
func TestSoftWrappedLineReportsWrapFlag(t *testing.T) {
	const cols, rows = 30, 10
	proc, err := nvimproc.Spawn("nvim", []string{"--clean", "-c", "set number wrap"}, nil, cols, rows)
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

	proc.Input("ihttps://example.com/a/very/long/path/that/wraps<Esc>")

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		snap := st.Snapshot()
		for id, gv := range snap.Grids {
			if id == 1 || len(gv.Data) < 2 || !strings.Contains(rowText(gv.Data[0]), "https://") {
				continue
			}
			if len(gv.Wrapped) > 0 && gv.Wrapped[0] && !gv.Wrapped[1] {
				return
			}
		}
	}
	dumpGrids(t, st.Snapshot())
	t.Fatal("no buffer grid reported row 0 as soft-wrapped")
}
