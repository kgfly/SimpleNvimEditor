// Package cli turns SimpleNvimEditor's command line into the handful of
// options the rest of the program actually needs.
//
// It deliberately depends on nothing but the standard library — no Gio, no
// Nvim, no config file — so the whole argument surface can be exercised as
// a pure function in unit tests instead of only through a real GUI launch.
package cli

import (
	"flag"
	"fmt"
	"io"
)

// Separator ends SimpleNvimEditor's own flags. Everything after it is
// handed to Nvim untouched, which is what lets a launcher write:
//
//	simplenvim --maximized -- -c term -c edit ~/notes.todo
//
// Without it, Nvim's own flags (`-c`, `-u`, `-O`, ...) would be parsed as
// ours and rejected.
const Separator = "--"

// Options is everything the command line can say.
type Options struct {
	// ShowVersion asks for the version string instead of a window.
	ShowVersion bool
	// NvimPath overrides the nvim executable named by the config file.
	NvimPath string
	// Maximized starts the window filling the monitor's work area.
	Maximized bool
	// NvimArgs are appended verbatim to the Nvim invocation: first the
	// pass-through arguments given after Separator, then any plain
	// positional arguments. See Parse for why that order.
	NvimArgs []string
}

// Split divides argv at the first Separator, returning the arguments this
// program parses itself and the ones destined for Nvim.
//
// Splitting before flag parsing (rather than relying on flag's own "--"
// handling) is what keeps the two groups distinguishable: the standard
// parser lumps everything after "--" in with ordinary positional files,
// which would lose the ordering guarantee Parse documents.
func Split(argv []string) (own, passthrough []string) {
	for i, arg := range argv {
		if arg == Separator {
			// Everything after the *first* separator is verbatim,
			// including any further "--" Nvim may want to receive.
			return argv[:i], argv[i+1:]
		}
	}
	return argv, nil
}

// Parse interprets argv (the arguments *after* the program name) for a
// program invoked as name, writing usage and errors to out.
//
// Pass-through arguments come before positional files in NvimArgs because
// some Nvim flags are positional-hungry: `simplenvim a.txt b.txt -- -O`
// must become `nvim -O a.txt b.txt`, not `nvim a.txt b.txt -O`. That
// ordering lives here, once, so no caller has to re-derive it.
//
// A flag.ErrHelp return means -h/--help was requested and usage has
// already been written to out; the caller should exit successfully.
func Parse(name string, argv []string, out io.Writer) (Options, error) {
	own, passthrough := Split(argv)

	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(out)

	var o Options
	fs.BoolVar(&o.ShowVersion, "version", false, "print version and exit")
	fs.StringVar(&o.NvimPath, "nvim", "", "path to the nvim executable (overrides config file)")
	fs.BoolVar(&o.Maximized, "maximized", false, "start with the window maximized")
	fs.Usage = func() {
		fmt.Fprintf(out, "Usage: %s [flags] [file...] [%s nvim-args...]\n\n", name, Separator)
		fs.PrintDefaults()
		fmt.Fprintf(out, "\nArguments after %q are passed to nvim unchanged, e.g.\n"+
			"  %s --maximized %s -c term -c edit notes.todo\n", Separator, name, Separator)
	}

	if err := fs.Parse(own); err != nil {
		return Options{}, err
	}

	files := fs.Args()
	o.NvimArgs = make([]string, 0, len(passthrough)+len(files))
	o.NvimArgs = append(o.NvimArgs, passthrough...)
	o.NvimArgs = append(o.NvimArgs, files...)
	return o, nil
}
