//go:build !linux

package nvimproc

import (
	"context"

	"github.com/neovim/go-client/nvim"
)

func startChild(ctx context.Context, command string, args []string) (*nvim.Nvim, func(), error) {
	v, err := nvim.NewChildProcess(
		nvim.ChildProcessCommand(command),
		nvim.ChildProcessArgs(args...),
		nvim.ChildProcessServe(false),
		nvim.ChildProcessContext(ctx),
	)
	return v, func() {}, err
}
