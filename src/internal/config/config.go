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

	"github.com/kgfly/SimpleNvimEditor/internal/input"
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
	// UseSystemFonts is set automatically when FontFamily differs from
	// the bundled default; users never need to set it.
	UseSystemFonts bool
	// FontFamily selects the monospace font. When set to something other
	// than the default "monospace", system font resolution is used
	// instead of the bundled Go Mono typeface.
	FontFamily string `toml:"font_family"`
	// AltIsMeta sends Alt/Option chords to Nvim as <A-...> mappings.
	//
	// A pointer so that "absent" is distinguishable from "explicitly
	// false": the default is true, which a plain bool could not express
	// through TOML's zero value. See input.Policy for what it controls
	// and why macOS is the only platform where turning it off is
	// meaningful (it costs you Option-composed characters like "å").
	AltIsMeta *bool `toml:"alt_is_meta"`
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
	if cfg.Editor.FontFamily != "" && cfg.Editor.FontFamily != Default().Editor.FontFamily {
		cfg.Editor.UseSystemFonts = true
	}
	return cfg, nil
}

// InputPolicy returns the key-handling policy implied by the config.
func (c EditorConfig) InputPolicy() input.Policy {
	p := input.DefaultPolicy()
	if c.AltIsMeta != nil {
		p.AltIsMeta = *c.AltIsMeta
	}
	return p
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

// DefaultFontFamilyForOS returns a reasonable monospace font family for
// the current OS, as a starting point for users who want to set font_family.
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
