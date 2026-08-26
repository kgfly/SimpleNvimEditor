package editorapp

import (
	"image"
	"testing"

	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
)

func TestAppIcon(t *testing.T) {
	img := appIcon()
	if img == nil {
		t.Fatal("appIcon returned nil")
	}
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		t.Fatalf("appIcon returned empty image: %v", b)
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

func TestSyncSizeWithZeroCellDims(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	// CellWidth and CellHeight are 0, should return early.
	a.syncSize(image.Pt(800, 600))
	if a.proc != nil {
		t.Fatal("syncSize should not start nvim with zero cell dims")
	}
}
