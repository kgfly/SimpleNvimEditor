package unit_test

import (
	"testing"

	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

func TestGridCursorGoto(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("grid_cursor_goto", args(3, 7, 2))))

	c := s.Snapshot().Cursor
	want := uistate.Cursor{GridID: 3, Row: 7, Col: 2}
	if c != want {
		t.Fatalf("cursor = %+v, want %+v", c, want)
	}
}

func TestModeInfoSetAndModeChange(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("mode_info_set", args(true, []interface{}{
		map[string]interface{}{
			"cursor_shape":    "block",
			"cell_percentage": 100,
			"attr_id":         0,
		},
		map[string]interface{}{
			"cursor_shape":    "horizontal",
			"cell_percentage": 25,
			"attr_id":         5,
		},
	}))))
	s.Apply(batch(ev("mode_change", args("insert", 1))))

	info := s.Snapshot().CurrentModeInfo()
	want := uistate.ModeInfo{CursorShape: "horizontal", CellPercentage: 25, AttrID: 5}
	if info != want {
		t.Fatalf("mode info = %+v, want %+v", info, want)
	}
}

func TestModeInfoDefaultsWhenFieldsMissing(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("mode_info_set", args(true, []interface{}{
		map[string]interface{}{}, // no fields at all
	}))))
	s.Apply(batch(ev("mode_change", args("normal", 0))))

	info := s.Snapshot().CurrentModeInfo()
	want := uistate.ModeInfo{CursorShape: "block", CellPercentage: 100, AttrID: 0}
	if info != want {
		t.Fatalf("mode info defaults = %+v, want %+v", info, want)
	}
}

func TestCurrentModeInfoFallsBackWhenNoModeChangeYet(t *testing.T) {
	s := uistate.New()
	info := s.Snapshot().CurrentModeInfo()
	want := uistate.ModeInfo{CursorShape: "block", CellPercentage: 100}
	if info != want {
		t.Fatalf("info before any mode_change = %+v, want %+v", info, want)
	}
}

func TestCurrentModeInfoFallsBackWhenIndexOutOfRange(t *testing.T) {
	s := uistate.New()
	s.Apply(batch(ev("mode_info_set", args(true, []interface{}{
		map[string]interface{}{"cursor_shape": "block", "cell_percentage": 100},
	}))))
	// mode_change points past the single mode_info_set entry.
	s.Apply(batch(ev("mode_change", args("cmdline_normal", 5))))

	info := s.Snapshot().CurrentModeInfo()
	want := uistate.ModeInfo{CursorShape: "block", CellPercentage: 100}
	if info != want {
		t.Fatalf("out-of-range mode info = %+v, want fallback %+v", info, want)
	}
}
