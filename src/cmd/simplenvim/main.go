// Command simplenvim is SimpleNvimEditor: a native Neovim front-end rendered
// with Gio. See IMPLEMENTATION_PLAN.md for the overall design and
// doc/developer.md for build/run instructions.
package main

import (
	"flag"
	"fmt"
	"os"

	gioapp "gioui.org/app"

	editorapp "github.com/kgfly/SimpleNvimEditor/internal/app"
	"github.com/kgfly/SimpleNvimEditor/internal/config"
)

func main() {
	nvimPath := flag.String("nvim", "", "path to the nvim executable (overrides config file)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [file...]\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "simplenvim: loading config: %v\n", err)
		os.Exit(1)
	}
	if *nvimPath != "" {
		cfg.Nvim.Command = *nvimPath
	}

	files := flag.Args()
	a := editorapp.New(cfg, files)

	go func() {
		win := new(gioapp.Window)
		if err := a.Run(win); err != nil {
			fmt.Fprintf(os.Stderr, "simplenvim: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	gioapp.Main()
}
