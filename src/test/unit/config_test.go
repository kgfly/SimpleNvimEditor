package unit_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
)

// withIsolatedConfigDir points os.UserConfigDir (used by config.Dir) at a
// fresh temp directory, so these tests never read or write a real user's
// config file, and works the same way across Linux/macOS/Windows.
func withIsolatedConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("AppData", dir)
	case "darwin":
		// os.UserConfigDir on darwin is $HOME/Library/Application Support.
		t.Setenv("HOME", dir)
	default:
		t.Setenv("XDG_CONFIG_HOME", dir)
	}
	return dir
}

func TestDefaultValues(t *testing.T) {
	d := config.Default()
	if d.Editor.FontSize != 14 {
		t.Errorf("default FontSize = %v, want 14", d.Editor.FontSize)
	}
	if d.Editor.FontFamily != "monospace" {
		t.Errorf("default FontFamily = %q, want %q", d.Editor.FontFamily, "monospace")
	}
	if d.Nvim.Command != "nvim" {
		t.Errorf("default Nvim.Command = %q, want %q", d.Nvim.Command, "nvim")
	}
	if len(d.Nvim.ExtraArgs) != 0 {
		t.Errorf("default Nvim.ExtraArgs = %v, want empty", d.Nvim.ExtraArgs)
	}
}

func TestLoadFallsBackToDefaultsWhenFileMissing(t *testing.T) {
	withIsolatedConfigDir(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	// Config embeds a []string field (Nvim.ExtraArgs), so it isn't
	// comparable with ==; compare the scalar fields we actually care about.
	want := config.Default()
	if cfg.Editor != want.Editor {
		t.Fatalf("Editor = %+v, want defaults %+v", cfg.Editor, want.Editor)
	}
	if cfg.Nvim.Command != want.Nvim.Command {
		t.Fatalf("Nvim.Command = %q, want %q", cfg.Nvim.Command, want.Nvim.Command)
	}
	if len(cfg.Nvim.ExtraArgs) != 0 {
		t.Fatalf("Nvim.ExtraArgs = %v, want empty", cfg.Nvim.ExtraArgs)
	}
}

func TestLoadOverlaysFileOnTopOfDefaults(t *testing.T) {
	withIsolatedConfigDir(t)

	path, err := config.FilePath()
	if err != nil {
		t.Fatalf("FilePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	toml := `
[editor]
font_size = 20
font_family = "Consolas"

[nvim]
command = "/opt/bin/nvim"
extra_args = ["--clean"]
`
	if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Editor.FontSize != 20 {
		t.Errorf("FontSize = %v, want 20", cfg.Editor.FontSize)
	}
	if !cfg.Editor.UseSystemFonts {
		t.Errorf("UseSystemFonts not auto-inferred from font_family")
	}
	if cfg.Editor.FontFamily != "Consolas" {
		t.Errorf("FontFamily = %q, want %q", cfg.Editor.FontFamily, "Consolas")
	}
	if cfg.Nvim.Command != "/opt/bin/nvim" {
		t.Errorf("Nvim.Command = %q, want %q", cfg.Nvim.Command, "/opt/bin/nvim")
	}
	if len(cfg.Nvim.ExtraArgs) != 1 || cfg.Nvim.ExtraArgs[0] != "--clean" {
		t.Errorf("Nvim.ExtraArgs = %v, want [--clean]", cfg.Nvim.ExtraArgs)
	}
}

func TestLoadFillsInZeroFontSizeAndCommand(t *testing.T) {
	withIsolatedConfigDir(t)

	path, err := config.FilePath()
	if err != nil {
		t.Fatalf("FilePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// A config file that only sets one field; the rest should fall back.
	toml := `
[editor]
font_family = "Iosevka"
`
	if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Editor.FontFamily != "Iosevka" {
		t.Errorf("FontFamily = %q, want %q", cfg.Editor.FontFamily, "Iosevka")
	}
	if cfg.Editor.FontSize != config.Default().Editor.FontSize {
		t.Errorf("FontSize = %v, want default %v", cfg.Editor.FontSize, config.Default().Editor.FontSize)
	}
	if cfg.Nvim.Command != config.Default().Nvim.Command {
		t.Errorf("Nvim.Command = %q, want default %q", cfg.Nvim.Command, config.Default().Nvim.Command)
	}
}

func TestLoadReturnsErrorOnInvalidTOML(t *testing.T) {
	withIsolatedConfigDir(t)

	path, err := config.FilePath()
	if err != nil {
		t.Fatalf("FilePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("this is not valid toml [[["), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := config.Load(); err == nil {
		t.Fatalf("Load() with malformed TOML returned nil error, want an error")
	}
}

func TestDirAndFilePath(t *testing.T) {
	withIsolatedConfigDir(t)

	dir, err := config.Dir()
	if err != nil {
		t.Fatalf("Dir() error = %v", err)
	}
	if filepath.Base(dir) != "simplenvimeditor" {
		t.Errorf("Dir() = %q, want it to end in %q", dir, "simplenvimeditor")
	}

	path, err := config.FilePath()
	if err != nil {
		t.Fatalf("FilePath() error = %v", err)
	}
	if filepath.Base(path) != "config.toml" {
		t.Errorf("FilePath() = %q, want it to end in %q", path, "config.toml")
	}
	if filepath.Dir(path) != dir {
		t.Errorf("FilePath() dir = %q, want it to match Dir() = %q", filepath.Dir(path), dir)
	}
}

func TestDefaultFontFamilyForOSNeverEmpty(t *testing.T) {
	if got := config.DefaultFontFamilyForOS(); got == "" {
		t.Fatalf("DefaultFontFamilyForOS() returned empty string")
	}
}

func TestLoadExplicitZeroFontSizeAndEmptyCommand(t *testing.T) {
	withIsolatedConfigDir(t)

	path, err := config.FilePath()
	if err != nil {
		t.Fatalf("FilePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	toml := `
[editor]
font_size = 0

[nvim]
command = ""
`
	if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Editor.FontSize != config.Default().Editor.FontSize {
		t.Errorf("zero FontSize should fall back to default, got %v", cfg.Editor.FontSize)
	}
	if cfg.Nvim.Command != config.Default().Nvim.Command {
		t.Errorf("empty Command should fall back to default, got %q", cfg.Nvim.Command)
	}
}

func TestLoadNegativeFontSize(t *testing.T) {
	withIsolatedConfigDir(t)

	path, err := config.FilePath()
	if err != nil {
		t.Fatalf("FilePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("[editor]\nfont_size = -5\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Editor.FontSize != config.Default().Editor.FontSize {
		t.Errorf("negative FontSize should fall back to default, got %v", cfg.Editor.FontSize)
	}
}

func TestLoadUnreadableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod does not restrict reads on Windows")
	}
	withIsolatedConfigDir(t)

	path, err := config.FilePath()
	if err != nil {
		t.Fatalf("FilePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("[editor]\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Skipf("cannot chmod on this platform: %v", err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) })

	_, err = config.Load()
	if err == nil {
		t.Fatal("Load() with unreadable file should return an error")
	}
}

func TestLoadSameFontFamilyAsDefault(t *testing.T) {
	withIsolatedConfigDir(t)

	path, err := config.FilePath()
	if err != nil {
		t.Fatalf("FilePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Set font_family to the same as default — UseSystemFonts should NOT be set.
	toml := `[editor]
font_family = "monospace"
`
	if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Editor.UseSystemFonts {
		t.Error("UseSystemFonts should not be set when font_family == default")
	}
}
