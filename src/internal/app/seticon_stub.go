//go:build !(((linux && !android) || freebsd || openbsd) && !nox11)

package editorapp

func setWindowTitle(_ any, _ string) {}
