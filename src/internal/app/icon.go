package editorapp

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	_ "image/png"
)

//go:embed icon.png
var iconPNG []byte

func appIcon() *image.RGBA {
	src, _, _ := image.Decode(bytes.NewReader(iconPNG))
	rgba, ok := src.(*image.RGBA)
	if ok {
		return rgba
	}
	b := src.Bounds()
	rgba = image.NewRGBA(b)
	draw.Draw(rgba, b, src, b.Min, draw.Src)
	return rgba
}
