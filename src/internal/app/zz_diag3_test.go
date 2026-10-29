package editorapp

import (
	"os"

	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
)

func nvimproc_Spawn(path, file string, cols, rows int) (*nvimproc.Process, error) {
	var args []string
	if os.Getenv("ZZNOFILE") == "" {
		args = append(args, file)
	}
	if lua := os.Getenv("ZZLUA"); lua != "" {
		args = append(args, "--cmd", "let $KGG_VIM_CODING_LSP='1'", "-c", lua)
	}
	return nvimproc.Spawn(path, nil, args, cols, rows)
}
