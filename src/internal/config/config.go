// Package config loads user-facing settings for SimpleNvimEditor from a TOML
// file, applying sane cross-platform defaults when no file (or no value) is
// present. Nvim itself remains the source of truth for editing behavior;
// everything here only configures the GUI shell.
package config

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

// Config holds all GUI-level settings.
type Config struct {
	Editor EditorConfig `toml:"editor"`
	Nvim   NvimConfig   `toml:"nvim"`
}

// EditorConfig controls font and rendering behavior.
type EditorConfig struct {
	// FontSize is the monospace grid font size, in points.
	FontSize float64 `toml:"font_size"`
	// UseSystemFonts, when true, asks the text shaper to resolve
	// FontFamily against fonts installed on the host OS instead of the
	// bundled Go Mono typeface. Bundled is the safer cross-platform
	// default because it never depends on what happens to be installed.
	UseSystemFonts bool `toml:"use_system_fonts"`
	// FontFamily is only consulted when UseSystemFonts is true.
	FontFamily string `toml:"font_family"`
}

// NvimConfig controls how the backend Nvim process is located and launched.
type NvimConfig struct {
	// Command is the nvim executable to run. Defaults to "nvim", resolved
	// via PATH, which works the same way on Linux, macOS, and Windows.
	Command string `toml:"command"`
	// ExtraArgs are appended verbatim to the nvim invocation.
	ExtraArgs []string `toml:"extra_args"`
}

// Default returns the built-in configuration used when no config file is
// present, or to fill in zero-valued fields from a partially specified file.
func Default() Config {
	return Config{
		Editor: EditorConfig{
			FontSize:       14,
			UseSystemFonts: false,
			FontFamily:     "monospace",
		},
		Nvim: NvimConfig{
			Command:   "nvim",
			ExtraArgs: nil,
		},
	}
}

// Load reads the config file from the platform-appropriate config directory
// (see Dir), overlaying it on top of Default. A missing file is not an
// error; it just means "use the defaults".
func Load() (Config, error) {
	cfg := Default()

	path, err := FilePath()
	if err != nil {
		// Not being able to determine a config dir is not fatal: fall
		// back to defaults rather than refusing to start the editor.
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return cfg, err
	}
	if cfg.Editor.FontSize <= 0 {
		cfg.Editor.FontSize = Default().Editor.FontSize
	}
	if cfg.Nvim.Command == "" {
		cfg.Nvim.Command = Default().Nvim.Command
	}
	return cfg, nil
}

// Dir returns the directory SimpleNvimEditor keeps its config file in,
// following each OS's usual convention via os.UserConfigDir.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "simplenvimeditor"), nil
}

// FilePath returns the full path to the config.toml file.
func FilePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// DefaultFontFamilyForOS is unused by Default (which favors the bundled
// font) but is kept as a documented starting point for users who opt into
// UseSystemFonts.
func DefaultFontFamilyForOS() string {
	switch runtime.GOOS {
	case "windows":
		return "Consolas"
	case "darwin":
		return "Menlo"
	default:
		return "monospace"
	}
}
