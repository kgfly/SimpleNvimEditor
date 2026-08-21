//go:build !(((linux && !android) || freebsd || openbsd) && !nox11)

package editorapp

import "image"

func setWindowIcon(_ any, _ *image.RGBA) {}
func setWindowTitle(_ any, _ string)     {}
