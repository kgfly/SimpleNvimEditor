package unit_test

import (
	"reflect"
	"testing"

	"github.com/kgfly/SimpleNvimEditor/internal/cli"
)

func TestParseForwardsNvimCommandsAfterBoundary(t *testing.T) {
	args := []string{
		"--maximized", "--", "-c", "term", "-c", "edit",
		"/Users/k0g0kfq/data1/.nnn/n.todo",
	}

	opts, err := cli.Parse(args)
	if err != nil {
		t.Fatalf("Parse(%q): %v", args, err)
	}
	if !opts.Maximized {
		t.Fatal("Maximized = false, want true")
	}
	want := []string{"-c", "term", "-c", "edit", "/Users/k0g0kfq/data1/.nnn/n.todo"}
	if !reflect.DeepEqual(opts.NvimArgs, want) {
		t.Fatalf("NvimArgs = %q, want %q", opts.NvimArgs, want)
	}
}

func TestParseAcceptsApplicationFlagsAndFiles(t *testing.T) {
	opts, err := cli.Parse([]string{"--nvim", "/custom/nvim", "notes.md"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if opts.NvimPath != "/custom/nvim" {
		t.Fatalf("NvimPath = %q, want /custom/nvim", opts.NvimPath)
	}
	if !reflect.DeepEqual(opts.NvimArgs, []string{"notes.md"}) {
		t.Fatalf("NvimArgs = %q, want [notes.md]", opts.NvimArgs)
	}
}

func TestParseRejectsNvimFlagsBeforeBoundary(t *testing.T) {
	if _, err := cli.Parse([]string{"-c", "term"}); err == nil {
		t.Fatal("Parse accepted Nvim flag before --")
	}
}
