package editorapp

import (
	"image"
	"testing"

	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
	"github.com/kgfly/SimpleNvimEditor/internal/render"
)

func TestAppIcon(t *testing.T) {
	for _, color := range iconColors {
		img := decodeIcon(iconPNG(color))
		if img == nil {
			t.Fatalf("%s icon decoded to nil", color)
		}
		if b := img.Bounds(); b.Dx() == 0 || b.Dy() == 0 {
			t.Fatalf("%s icon is empty: %v", color, b)
		}
	}
}

func TestStartupIconColor(t *testing.T) {
	tests := []struct {
		name    string
		environ []string
		args    []string
		want    string
		fromEnv bool
	}{
		{"default", nil, nil, "blue", false},
		{"file", nil, []string{"notes.md"}, "blue", false},
		{"term", nil, []string{"-c", "term", "-c", "startinsert"}, "green", false},
		{"terminal with cmd", nil, []string{"-c", "terminal htop"}, "green", false},
		{"plus ter", nil, []string{"+ter"}, "green", false},
		{"too short", nil, []string{"-c", "te"}, "blue", false},
		{"not terminal", nil, []string{"-c", "termx"}, "blue", false},
		{"term then file", nil, []string{"-c", "term", "notes.md"}, "blue", false},
		{"term after --", nil, []string{"-c", "term", "--"}, "green", false},
		{"env wins", []string{"SIMPLENVIM_BG_RED=1"}, []string{"-c", "term"}, "red", true},
		{"env lowercase", []string{"simplenvim_bg_pink="}, nil, "pink", true},
		{"env unknown", []string{"SIMPLENVIM_BG_TEAL=1"}, nil, "blue", false},
		{"env order", []string{"SIMPLENVIM_BG_GRAY=1", "SIMPLENVIM_BG_YELLOW=1"}, nil, "yellow", true},
	}
	for _, tt := range tests {
		got, fromEnv := startupIconColor(tt.environ, tt.args)
		if got != tt.want || fromEnv != tt.fromEnv {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", tt.name, got, fromEnv, tt.want, tt.fromEnv)
		}
	}
}

func TestNewApp(t *testing.T) {
	cfg := config.Default()
	a := New(cfg, []string{"file1.go", "file2.go"}, Options{Maximized: true})
	if a == nil {
		t.Fatal("New returned nil")
	}
	if a.state == nil {
		t.Fatal("New should initialize state")
	}
	if !a.options.Maximized {
		t.Fatal("New should retain window options")
	}
	if len(a.nvimArgs) != 2 {
		t.Fatalf("nvimArgs = %v, want 2 entries", a.nvimArgs)
	}
}

func TestQuitWithNilProc(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	// Should not panic.
	a.quit()
}

func TestOnKeyWithNilProc(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	a.onKey(key.Event{Name: "A", State: key.Press})
}

func TestOnEditWithNilProc(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	a.onEdit(key.EditEvent{Text: "hello"})
}

func TestOnPointerWithNilProc(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	a.onPointer(pointer.Event{Kind: pointer.Press, Buttons: pointer.ButtonPrimary})
}

func TestCaretWithZeroMetrics(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	c := a.caret()
	if c.Ascent != 0 || c.Descent != 0 {
		t.Fatalf("caret with zero metrics should be zero, got %+v", c)
	}
}

func TestCaretWithMissingGrid(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	a.fonts.Metrics = render.Metrics{CellWidth: 8, CellHeight: 16, Baseline: 12}

	if got := a.caret(); got != (key.Caret{}) {
		t.Fatalf("caret with missing grid = %+v, want zero", got)
	}
}

func TestCaretPosition(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	a.fonts.Metrics = render.Metrics{CellWidth: 8, CellHeight: 16, Baseline: 12}
	a.state.Apply([][]interface{}{
		{"grid_resize", []interface{}{1, 10, 5}},
		{"grid_cursor_goto", []interface{}{1, 2, 3}},
	})

	got := a.caret()
	if got.Pos.X != 24 || got.Pos.Y != 44 || got.Ascent != 12 || got.Descent != 4 {
		t.Fatalf("caret = %+v, want position (24,44), ascent 12, descent 4", got)
	}
}

func TestSyncSizeWithZeroCellDims(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	// CellWidth and CellHeight are 0, should return early.
	a.syncSize(image.Pt(800, 600))
	if a.proc != nil {
		t.Fatal("syncSize should not start nvim with zero cell dims")
	}
}
