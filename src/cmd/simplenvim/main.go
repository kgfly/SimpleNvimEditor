// Command simplenvim is SimpleNvimEditor: a native Neovim front-end rendered
// with Gio. See IMPLEMENTATION_PLAN.md for the overall design and
// doc/developer.md for build/run instructions.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	gioapp "gioui.org/app"

	editorapp "github.com/kgfly/SimpleNvimEditor/internal/app"
	"github.com/kgfly/SimpleNvimEditor/internal/cli"
	"github.com/kgfly/SimpleNvimEditor/internal/config"
)

// Set at build time via -ldflags "-X main.version=1.0.0".
var version = "dev"

func main() {
	opts, err := cli.Parse(filepath.Base(os.Args[0]), os.Args[1:], os.Stderr)
	if err != nil {
		// -h/--help is a successful request for usage, which Parse has
		// already written; anything else is a real misuse and flag has
		// already reported the specific problem.
		if errors.Is(err, flag.ErrHelp) {
			return
		}
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

	a := editorapp.New(cfg, editorapp.Options{
		NvimArgs:  opts.NvimArgs,
		Maximized: opts.Maximized,
	})

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
