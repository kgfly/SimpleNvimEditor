package unit_test

import (
	"flag"
	"io"
	"reflect"
	"strings"
	"testing"

	gioapp "gioui.org/app"
	"gioui.org/unit"

	appkg "github.com/kgfly/SimpleNvimEditor/internal/app"
	"github.com/kgfly/SimpleNvimEditor/internal/cli"
)

// parseArgs runs cli.Parse the way main does, discarding usage output.
func parseArgs(t *testing.T, argv ...string) cli.Options {
	t.Helper()
	opts, err := cli.Parse("simplenvim", argv, io.Discard)
	if err != nil {
		t.Fatalf("Parse(%q) error = %v, want nil", argv, err)
	}
	return opts
}

// TestParseLauncherInvocation is the regression test for the exact command
// line a desktop launcher uses:
//
//	simplenvim --maximized -- -c term -c edit /path/to/n.todo
//
// Before pass-through support existed, everything after --maximized was
// parsed as our own flags, so "-c" aborted startup instead of reaching Nvim.
func TestParseLauncherInvocation(t *testing.T) {
	const todo = "/Users/k0g0kfq/data1/.nnn/n.todo"
	opts := parseArgs(t, "--maximized", "--", "-c", "term", "-c", "edit", todo)

	if !opts.Maximized {
		t.Error("Maximized = false, want true")
	}
	want := []string{"-c", "term", "-c", "edit", todo}
	if !reflect.DeepEqual(opts.NvimArgs, want) {
		t.Errorf("NvimArgs = %q, want %q", opts.NvimArgs, want)
	}
}

func TestParseArgumentForms(t *testing.T) {
	cases := []struct {
		desc      string
		argv      []string
		maximized bool
		nvimPath  string
		version   bool
		nvimArgs  []string
	}{
		{
			desc:     "no arguments at all opens an empty editor",
			argv:     nil,
			nvimArgs: []string{},
		},
		{
			desc:     "bare files are passed to nvim",
			argv:     []string{"a.txt", "b.txt"},
			nvimArgs: []string{"a.txt", "b.txt"},
		},
		{
			desc:      "single-dash and double-dash spell the same flag",
			argv:      []string{"-maximized"},
			maximized: true,
			nvimArgs:  []string{},
		},
		{
			desc:      "--maximized=false explicitly opts out",
			argv:      []string{"--maximized=false"},
			maximized: false,
			nvimArgs:  []string{},
		},
		{
			desc:     "the -nvim override is ours, not nvim's",
			argv:     []string{"--nvim", "/opt/bin/nvim", "x.txt"},
			nvimPath: "/opt/bin/nvim",
			nvimArgs: []string{"x.txt"},
		},
		{
			desc:     "--version is recognised",
			argv:     []string{"--version"},
			version:  true,
			nvimArgs: []string{},
		},
		{
			// Nvim flags are positional-hungry ("nvim -O a b" opens
			// splits), so pass-through must precede files.
			desc:     "pass-through args come before positional files",
			argv:     []string{"a.txt", "b.txt", "--", "-O"},
			nvimArgs: []string{"-O", "a.txt", "b.txt"},
		},
		{
			desc:     "a separator with nothing after it is harmless",
			argv:     []string{"a.txt", "--"},
			nvimArgs: []string{"a.txt"},
		},
		{
			// Only the first separator is ours to consume; a second
			// one is meaningful to nvim (it ends *its* option list).
			desc:     "later separators are passed through verbatim",
			argv:     []string{"--", "-u", "NONE", "--", "-weird-file"},
			nvimArgs: []string{"-u", "NONE", "--", "-weird-file"},
		},
		{
			desc:      "our flags and pass-through args combine",
			argv:      []string{"--maximized", "--nvim", "/n", "--", "-c", "term"},
			maximized: true,
			nvimPath:  "/n",
			nvimArgs:  []string{"-c", "term"},
		},
		{
			// Without the separator this would be a parse error, so
			// it doubles as proof the separator is what rescues it.
			desc:     "nvim flags survive verbatim after the separator",
			argv:     []string{"--", "-c", "set nonumber", "--cmd", "let g:x=1"},
			nvimArgs: []string{"-c", "set nonumber", "--cmd", "let g:x=1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			opts := parseArgs(t, tc.argv...)
			if opts.Maximized != tc.maximized {
				t.Errorf("Maximized = %v, want %v", opts.Maximized, tc.maximized)
			}
			if opts.NvimPath != tc.nvimPath {
				t.Errorf("NvimPath = %q, want %q", opts.NvimPath, tc.nvimPath)
			}
			if opts.ShowVersion != tc.version {
				t.Errorf("ShowVersion = %v, want %v", opts.ShowVersion, tc.version)
			}
			if !reflect.DeepEqual(opts.NvimArgs, tc.nvimArgs) {
				t.Errorf("NvimArgs = %q, want %q", opts.NvimArgs, tc.nvimArgs)
			}
		})
	}
}

// TestParseUnknownFlagIsAnError guards the other half of the separator's
// value: without it an nvim-style flag really is rejected, so callers
// genuinely need "--" rather than it being cosmetic.
func TestParseUnknownFlagIsAnError(t *testing.T) {
	var out strings.Builder
	if _, err := cli.Parse("simplenvim", []string{"-c", "term"}, &out); err == nil {
		t.Fatal("Parse(-c term) returned nil error, want a parse failure")
	}
	if out.Len() == 0 {
		t.Error("a parse failure should explain itself on the provided writer")
	}
}

// TestParseHelpIsNotAFailure pins the distinction main relies on: "user
// asked for help" (exit 0) versus "user got it wrong" (exit 2).
func TestParseHelpIsNotAFailure(t *testing.T) {
	var out strings.Builder
	_, err := cli.Parse("simplenvim", []string{"-h"}, &out)
	if err != flag.ErrHelp {
		t.Fatalf("Parse(-h) error = %v, want flag.ErrHelp", err)
	}
	// The separator is the non-obvious part of the interface, so usage
	// has to actually document it.
	usage := out.String()
	for _, want := range []string{"Usage:", cli.Separator, "maximized"} {
		if !strings.Contains(usage, want) {
			t.Errorf("usage text does not mention %q:\n%s", want, usage)
		}
	}
}

func TestSplitFindsTheFirstSeparator(t *testing.T) {
	cases := []struct {
		desc             string
		argv             []string
		own, passthrough []string
	}{
		{"no separator", []string{"a", "-maximized"}, []string{"a", "-maximized"}, nil},
		{"separator first", []string{"--", "-c", "q"}, []string{}, []string{"-c", "q"}},
		{"separator last", []string{"a", "--"}, []string{"a"}, []string{}},
		{"repeated separators", []string{"--", "--", "x"}, []string{}, []string{"--", "x"}},
		{"empty argv", nil, nil, nil},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			own, pass := cli.Split(tc.argv)
			if !reflect.DeepEqual(own, tc.own) {
				t.Errorf("own = %q, want %q", own, tc.own)
			}
			if !reflect.DeepEqual(pass, tc.passthrough) {
				t.Errorf("passthrough = %q, want %q", pass, tc.passthrough)
			}
		})
	}
}

// applyWindowOptions applies the App's real option set to a Gio Config the
// same way gioui.org/app does internally, so these tests exercise the
// production options rather than a restatement of them.
func applyWindowOptions(t *testing.T, opts appkg.Options) gioapp.Config {
	t.Helper()
	var cfg gioapp.Config
	metric := unit.Metric{PxPerDp: 1, PxPerSp: 1}
	for _, o := range appkg.WindowOptions(opts) {
		o(metric, &cfg)
	}
	return cfg
}

// TestWindowOptionsMaximized checks that --maximized reaches Gio as a real
// window mode, rather than being parsed and then quietly ignored.
func TestWindowOptionsMaximized(t *testing.T) {
	cfg := applyWindowOptions(t, appkg.Options{Maximized: true})
	if cfg.Mode != gioapp.Maximized {
		t.Errorf("Mode = %v, want %v", cfg.Mode, gioapp.Maximized)
	}
	// A restore size must still be requested: it is what the window
	// returns to when un-maximized, and the fallback on platforms that
	// cannot honor a maximize request at all.
	if cfg.Size.X <= 0 || cfg.Size.Y <= 0 {
		t.Errorf("Size = %v, want a positive restore size alongside Maximized", cfg.Size)
	}
}

func TestWindowOptionsDefaultsToWindowed(t *testing.T) {
	cfg := applyWindowOptions(t, appkg.Options{})
	if cfg.Mode != gioapp.Windowed {
		t.Errorf("Mode = %v, want %v", cfg.Mode, gioapp.Windowed)
	}
	if cfg.Size.X <= 0 || cfg.Size.Y <= 0 {
		t.Errorf("Size = %v, want a positive default window size", cfg.Size)
	}
}
