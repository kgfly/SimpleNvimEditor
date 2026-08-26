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
	NvimArgs    []string
}

// Parse parses simplenvim arguments. Application flags must precede --;
// arguments after that boundary are passed to Nvim unchanged.
func Parse(args []string) (Options, error) {
	var opts Options
	var usage bytes.Buffer

	flags := flag.NewFlagSet("simplenvim", flag.ContinueOnError)
	flags.SetOutput(&usage)
	flags.BoolVar(&opts.ShowVersion, "version", false, "print version and exit")
	flags.StringVar(&opts.NvimPath, "nvim", "", "path to the nvim executable (overrides config file)")
	flags.BoolVar(&opts.Maximized, "maximized", false, "start the editor window maximized")
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
