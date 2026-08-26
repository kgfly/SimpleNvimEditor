package integration_test

import (
	"path/filepath"
	"testing"

	"github.com/kgfly/SimpleNvimEditor/internal/cli"
	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
)

func TestCLIForwardsCommandsToNvimInOrder(t *testing.T) {
	file := filepath.Join(t.TempDir(), "n.todo")
	args := []string{"--maximized", "--", "-u", "NONE", "-n", "-c", "let g:cli_term = 1", "-c", "edit " + file}

	opts, err := cli.Parse(args)
	if err != nil {
		t.Fatalf("cli.Parse(%q): %v", args, err)
	}
	if !opts.Maximized {
		t.Fatal("Maximized = false, want true")
	}

	proc, err := nvimproc.Spawn(requireNvim(t), nil, opts.NvimArgs, 40, 10)
	if err != nil {
		t.Fatalf("nvimproc.Spawn: %v", err)
	}
	t.Cleanup(func() {
		proc.RequestQuit()
		<-proc.Exited
	})

	value, err := proc.Nvim.CommandOutput("echo get(g:, 'cli_term', 0)")
	if err != nil {
		t.Fatalf("query command result: %v", err)
	}
	if value != "1" {
		t.Fatalf("first -c command value = %q, want 1", value)
	}
	name, err := proc.Nvim.CommandOutput("echo expand('%:p')")
	if err != nil {
		t.Fatalf("query edited file: %v", err)
	}
	if name != file {
		t.Fatalf("edited file = %q, want %q", name, file)
	}
}
