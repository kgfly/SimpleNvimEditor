// Package cli parses arguments owned by simplenvim and preserves arguments
// intended for the Neovim child process.
package cli

import (
	"bytes"
	"flag"
	"fmt"
)

// Options holds simplenvim flags and the opaque arguments forwarded to Nvim.
type Options struct {
	ShowVersion bool
	NvimPath    string
	Maximized   bool
	// Slot is the instance number to start in, 0 for the lowest free one.
	Slot int
	// CheckSlot asks only whether slot CheckSlot is free, 0 for no check.
	CheckSlot int
	NvimArgs  []string
}

// Parse parses simplenvim arguments. Application flags must precede --;
// arguments after that boundary are passed to Nvim unchanged.
func Parse(args []string) (Options, error) {
	var opts Options
	var usage bytes.Buffer

	flags := flag.NewFlagSet("simplenvim", flag.ContinueOnError)
	flags.SetOutput(&usage)
	flags.BoolVar(&opts.ShowVersion, "version", false, "print version and exit")
	flags.BoolVar(&opts.ShowVersion, "v", false, "shorthand for --version")
	flags.StringVar(&opts.NvimPath, "nvim", "", "path to the nvim executable (overrides config file)")
	flags.BoolVar(&opts.Maximized, "maximized", false, "start the editor window maximized")
	flags.IntVar(&opts.Slot, "slot", 0, "start in this icon number (1-99) of the SIMPLENVIM_BG_<COLOR> group; fail if it is in use")
	flags.IntVar(&opts.CheckSlot, "check-slot", 0, "exit 0 if --slot `n` could be taken now, else print why and exit 1")
	flags.Usage = func() {
		fmt.Fprint(&usage, "Usage: simplenvim [flags] [file...]\n\n")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return Options{}, fmt.Errorf("parse arguments: %w\n%s", err, usage.String())
	}
	opts.NvimArgs = append([]string(nil), flags.Args()...)
	return opts, nil
}
