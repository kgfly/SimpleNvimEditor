package editorapp

import (
	"bufio"
	"fmt"
	"image"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
	"github.com/kgfly/SimpleNvimEditor/internal/render"
)

func TestAppIcon(t *testing.T) {
	for _, color := range iconColors {
		for n := 1; n <= maxSlot; n++ {
			icon := color
			if n > 1 {
				icon = fmt.Sprintf("%s_%d", color, n)
			}
			if _, err := iconFiles.ReadFile("icons/icon_bg_" + icon + ".png"); err != nil {
				t.Fatalf("%s icon missing: %v", icon, err)
			}
			img := decodeIcon(iconPNG(icon))
			if img == nil {
				t.Fatalf("%s icon decoded to nil", icon)
			}
			if b := img.Bounds(); b.Dx() == 0 || b.Dy() == 0 {
				t.Fatalf("%s icon is empty: %v", icon, b)
			}
		}
	}
}

func TestAppID(t *testing.T) {
	for icon, want := range map[string]string{
		"black":   "simplenvim",
		"black_2": "simplenvim-black-2",
		"blue":    "simplenvim-blue",
		"green":   "simplenvim-green",
		"green_9": "simplenvim-green-9",
	} {
		if got := appID(icon); got != want {
			t.Errorf("appID(%q) = %q, want %q", icon, got, want)
		}
	}
}

// TestClaimIconHelper is the child process for TestClaimIconAcrossProcesses:
// it claims a slot, reports the icon, and holds the slot until killed.
func TestClaimIconHelper(t *testing.T) {
	group := os.Getenv("SIMPLENVIM_TEST_SLOT_GROUP")
	if group == "" {
		t.Skip("helper process only")
	}
	fmt.Println(claimIcon(group))
	time.Sleep(time.Minute)
}

func TestClaimIconAcrossProcesses(t *testing.T) {
	switch runtime.GOOS {
	case "windows", "linux", "darwin":
	default:
		t.Skip("no slot claim on " + runtime.GOOS)
	}
	group := fmt.Sprintf("test%d", os.Getpid())
	start := func() (*exec.Cmd, string) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestClaimIconHelper$")
		cmd.Env = append(os.Environ(), "SIMPLENVIM_TEST_SLOT_GROUP="+group)
		out, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		})
		line, err := bufio.NewReader(out).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		return cmd, strings.TrimSpace(line)
	}
	kill := func(cmd *exec.Cmd) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}

	first, got := start()
	if got != group {
		t.Fatalf("first instance = %q, want %q", got, group)
	}
	if _, got := start(); got != group+"_2" {
		t.Fatalf("second instance = %q, want %q", got, group+"_2")
	}

	// A killed instance's slot is freed by the OS; macOS frees it
	// asynchronously, so allow a moment.
	kill(first)
	for i := 0; ; i++ {
		cmd, got := start()
		if got == group {
			break
		}
		kill(cmd)
		if i == 20 {
			t.Fatalf("after kill = %q, want %q", got, group)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestStartupIcon(t *testing.T) {
	tests := []struct {
		name     string
		environ  []string
		want     string
		numbered bool
	}{
		{"default", nil, "black", false},
		{"env default", []string{"SIMPLENVIM_BG_DEFAULT=1"}, "black", true},
		{"env black", []string{"SIMPLENVIM_BG_BLACK=1"}, "black", true},
		{"env", []string{"SIMPLENVIM_BG_RED=1"}, "red", true},
		{"env lowercase", []string{"simplenvim_bg_pink="}, "pink", true},
		{"env unknown", []string{"SIMPLENVIM_BG_TEAL=1"}, "black", false},
		{"env order", []string{"SIMPLENVIM_BG_GRAY=1", "SIMPLENVIM_BG_YELLOW=1"}, "yellow", true},
		{"env color over default", []string{"SIMPLENVIM_BG_DEFAULT=1", "SIMPLENVIM_BG_BLUE=1"}, "blue", true},
	}
	for _, tt := range tests {
		if got, numbered := startupIcon(tt.environ); got != tt.want || numbered != tt.numbered {
			t.Errorf("%s: got %q, %v; want %q, %v", tt.name, got, numbered, tt.want, tt.numbered)
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
