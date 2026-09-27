// Command icongen writes icon_bg_<color>_<n>.png (n = 2..9): the base
// icon_bg_<color>.png background with a big blocky <n> on it and a small
// "s" in the bottom-left corner, in the style of the base icon's "S".
//
//	go run ./icongen [dir [file]]   (dir defaults to icons; file writes only that one)
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
)

var baseName = regexp.MustCompile(`^icon_bg_[a-z]+\.png$`)

const ss = 4 // supersampling per pixel side

// style places a glyph box and sizes its strokes, in 64ths of the icon.
type style struct{ x, y, w, h, bar, stem, chamfer float64 }

var (
	digitStyle = style{x: 25, y: 8, w: 32, h: 48, bar: 11, stem: 10, chamfer: 4}
	smallS     = style{x: 7, y: 36, w: 14, h: 20, bar: 4, stem: 4, chamfer: 1.5}
)

func main() {
	dir, only := "icons", ""
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if len(os.Args) > 2 {
		only = os.Args[2]
	}
	paths, err := filepath.Glob(filepath.Join(dir, "icon_bg_*.png"))
	if err != nil {
		log.Fatal(err)
	}
	for _, path := range paths {
		if !baseName.MatchString(filepath.Base(path)) {
			continue
		}
		base, err := readPNG(path)
		if err != nil {
			log.Fatal(err)
		}
		bg, fg := iconColors(base)
		for n := 2; n <= 9; n++ {
			out := path[:len(path)-len(".png")] + fmt.Sprintf("_%d.png", n)
			if only != "" && filepath.Base(out) != only {
				continue
			}
			img := background(base, bg)
			mask := labelMask(img.Bounds().Dx(), placed{smallS, 'S'}, placed{digitStyle, rune('0' + n)})
			draw.DrawMask(img, img.Bounds(), image.NewUniform(fg), image.Point{}, mask, image.Point{}, draw.Over)
			if err := writePNG(out, img); err != nil {
				log.Fatal(err)
			}
		}
	}
}

// iconColors returns the base icon's background (sampled at the top edge)
// and its "S" color (the most common other opaque color).
func iconColors(base image.Image) (bg, fg color.NRGBA) {
	b := base.Bounds()
	bg = color.NRGBAModel.Convert(base.At(b.Min.X+b.Dx()/2, b.Min.Y+1)).(color.NRGBA)
	counts := map[color.NRGBA]int{}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if c := color.NRGBAModel.Convert(base.At(x, y)).(color.NRGBA); c.A == 0xff && c != bg {
				counts[c]++
			}
		}
	}
	best := 0
	for c, n := range counts {
		if n > best {
			fg, best = c, n
		}
	}
	return bg, fg
}

// background copies base with every opaque pixel painted bg, erasing the "S"
// but keeping the anti-aliased rounded corners.
func background(base image.Image, bg color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(base.Bounds())
	draw.Draw(img, img.Bounds(), base, base.Bounds().Min, draw.Src)
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i+3] == 0xff {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2] = bg.R, bg.G, bg.B
		}
	}
	return img
}

type rect struct{ x0, y0, x1, y1 float64 }

// glyph returns r's bars and stems relative to the glyph box; sharp ones
// keep square corners.
func (st style) glyph(r rune) (rects, sharp []rect) {
	w, h, b, s := st.w, st.h, st.bar, st.stem
	my := (h - b) / 2
	top := rect{0, 0, w, b}
	mid := rect{0, my, w, my + b}
	bot := rect{0, h - b, w, h}
	lu := rect{0, 0, s, my + b}
	ll := rect{0, my, s, h}
	ru := rect{w - s, 0, w, my + b}
	rl := rect{w - s, my, w, h}
	switch r {
	case 'S':
		return []rect{top, lu, mid, rl, bot}, nil
	case '2':
		return []rect{top, ru, mid, ll, bot}, nil
	case '3':
		return []rect{top, ru, rl, bot, {w * 0.3, my, w, my + b}}, nil
	case '4':
		return []rect{lu, mid, ru, rl}, nil
	case '5':
		return []rect{lu, mid, rl, bot}, []rect{top} // the square top tells it from S
	case '6':
		return []rect{top, lu, ll, mid, rl, bot}, nil
	case '7':
		return []rect{top, ru, rl}, nil
	case '8':
		return []rect{top, lu, ll, ru, rl, mid, bot}, nil
	case '9':
		return []rect{top, lu, ru, rl, mid, bot}, nil
	}
	log.Fatalf("no glyph for %q", r)
	return nil, nil
}

type placed struct {
	st style
	r  rune
}

// labelMask returns the anti-aliased coverage of the glyphs on a size-wide icon.
func labelMask(size int, glyphs ...placed) *image.Alpha {
	scale := float64(size) / 64 * ss
	n := size * ss
	on := make([]bool, n*n)
	for _, g := range glyphs {
		buf, sharp := make([]bool, n*n), make([]bool, n*n)
		rects, sharpRects := g.st.glyph(g.r)
		for _, rc := range rects {
			fillRect(buf, n, rc, g.st, scale)
		}
		for _, rc := range sharpRects {
			fillRect(sharp, n, rc, g.st, scale)
		}
		// Opening with a diamond cuts convex corners at 45° and keeps concave ones.
		k := int(math.Round(g.st.chamfer * scale))
		buf = morph(morph(buf, n, k, true), n, k, false)
		for i := range on {
			on[i] = on[i] || buf[i] || sharp[i]
		}
	}

	m := image.NewAlpha(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			hits := 0
			for sy := y * ss; sy < (y+1)*ss; sy++ {
				for sx := x * ss; sx < (x+1)*ss; sx++ {
					if on[sy*n+sx] {
						hits++
					}
				}
			}
			m.SetAlpha(x, y, color.Alpha{uint8(hits * 255 / (ss * ss))})
		}
	}
	return m
}

func fillRect(buf []bool, n int, r rect, st style, scale float64) {
	px := func(v float64) int { return int(math.Round(v * scale)) }
	for y := px(st.y + r.y0); y < px(st.y+r.y1); y++ {
		for x := px(st.x + r.x0); x < px(st.x+r.x1); x++ {
			buf[y*n+x] = true
		}
	}
}

// morph erodes (or dilates) buf k times by a 4-neighbor cross, which
// together is a diamond of radius k.
func morph(buf []bool, n, k int, erode bool) []bool {
	for ; k > 0; k-- {
		out := make([]bool, len(buf))
		for y := 0; y < n; y++ {
			for x := 0; x < n; x++ {
				v := buf[y*n+x]
				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, ny := x+d[0], y+d[1]
					nv := nx >= 0 && ny >= 0 && nx < n && ny < n && buf[ny*n+nx]
					if erode {
						v = v && nv
					} else {
						v = v || nv
					}
				}
				out[y*n+x] = v
			}
		}
		buf = out
	}
	return buf
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
