// Command simplenvim is SimpleNvimEditor: a native Neovim front-end rendered
// with Gio. See IMPLEMENTATION_PLAN.md for the overall design and
// doc/developer.md for build/run instructions.
package main

import (
	"fmt"
	"os"

	gioapp "gioui.org/app"

	editorapp "github.com/kgfly/SimpleNvimEditor/internal/app"
	"github.com/kgfly/SimpleNvimEditor/internal/cli"
	"github.com/kgfly/SimpleNvimEditor/internal/config"
)

// Set at build time via -ldflags "-X main.version=1.0.0".
var version = "dev"

func main() {
	opts, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "simplenvim: %v\n", err)
		os.Exit(2)
	}

	if opts.ShowVersion {
		fmt.Println(version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "simplenvim: loading config: %v\n", err)
		os.Exit(1)
	}
	if opts.NvimPath != "" {
		cfg.Nvim.Command = opts.NvimPath
	}

	a := editorapp.New(cfg, opts.NvimArgs, editorapp.Options{Maximized: opts.Maximized})

	// Subscribe to "open this document" requests from the desktop
	// environment before the event loop starts. On macOS the request for
	// the file that caused the launch arrives during startup, so a later
	// subscription would miss it and the app would open empty.
	editorapp.InstallOpenFileHandler()

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
