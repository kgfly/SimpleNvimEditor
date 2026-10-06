//go:build linux

package nvimproc

import (
	"context"
	"log"
	"os/exec"

	"github.com/neovim/go-client/nvim"
)

func startChild(ctx context.Context, command string, args []string) (*nvim.Nvim, func(), error) {
	cmd := exec.CommandContext(ctx, command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, nil, err
	}
	v, err := nvim.New(stdout, stdin, stdin, log.Printf)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, nil, err
	}
	go func() {
		<-ctx.Done()
		_ = stdout.Close()
	}()
	return v, func() { _ = cmd.Wait() }, nil
}
