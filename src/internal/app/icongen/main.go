// Command icongen writes icon_bg_<color>_<n>.png (n = 2..9): the base
// icon_bg_<color>.png background with "S<n>" on it, drawn here as a narrow
// version of the base icon's blocky "S", both glyphs the same size.
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
	"strconv"
)

var baseName = regexp.MustCompile(`^icon_bg_[a-z]+\.png$`)

// Glyph layout in 64ths of the icon: the base S's rows, bar height and
// corner cut, but two 24-wide columns with narrower stems.
const (
	glyphTop  = 9
	glyphH    = 46
	glyphLeft = 6
	glyphW    = 24
	glyphGap  = 4
	barH      = 10
	stemW     = 8
	midY      = 18
	chamfer   = 3.5
	ss        = 4 // supersampling per pixel side
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
			mask := labelMask("S"+strconv.Itoa(n), img.Bounds().Dx())
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

// The bars and stems glyphs are built from, relative to the glyph box.
var (
	top = rect{0, 0, glyphW, barH}
	mid = rect{0, midY, glyphW, midY + barH}
	bot = rect{0, glyphH - barH, glyphW, glyphH}
	lu  = rect{0, 0, stemW, midY + barH}
	ll  = rect{0, midY, stemW, glyphH}
	ru  = rect{glyphW - stemW, 0, glyphW, midY + barH}
	rl  = rect{glyphW - stemW, midY, glyphW, glyphH}
)

// glyph is a character's rects; sharp ones keep square corners.
type glyph struct{ rects, sharp []rect }

var glyphs = map[rune]glyph{
	'S': {rects: []rect{top, lu, mid, rl, bot}},
	'2': {rects: []rect{top, ru, mid, ll, bot}},
	'3': {rects: []rect{top, ru, rl, bot, {glyphW * 0.3, midY, glyphW, midY + barH}}},
	'4': {rects: []rect{lu, mid, ru, rl}},
	'5': {rects: []rect{lu, mid, rl, bot}, sharp: []rect{top}}, // the square top tells it from S
	'6': {rects: []rect{top, lu, ll, mid, rl, bot}},
	'7': {rects: []rect{top, ru, rl}},
	'8': {rects: []rect{top, lu, ll, ru, rl, mid, bot}},
	'9': {rects: []rect{top, lu, ru, rl, mid, bot}},
}

// labelMask returns the anti-aliased coverage of label on a size-wide icon.
func labelMask(label string, size int) *image.Alpha {
	scale := float64(size) / 64 * ss
	n := size * ss
	on, sharp := make([]bool, n*n), make([]bool, n*n)
	for i, r := range label {
		ox := glyphLeft + float64(i)*(glyphW+glyphGap)
		for _, rc := range glyphs[r].rects {
			fillRect(on, n, rc, ox, scale)
		}
		for _, rc := range glyphs[r].sharp {
			fillRect(sharp, n, rc, ox, scale)
		}
	}
	// Opening with a diamond cuts convex corners at 45° and keeps concave ones.
	k := int(math.Round(chamfer * scale))
	on = morph(morph(on, n, k, true), n, k, false)

	m := image.NewAlpha(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			hits := 0
			for sy := y * ss; sy < (y+1)*ss; sy++ {
				for sx := x * ss; sx < (x+1)*ss; sx++ {
					if on[sy*n+sx] || sharp[sy*n+sx] {
						hits++
					}
				}
			}
			m.SetAlpha(x, y, color.Alpha{uint8(hits * 255 / (ss * ss))})
		}
	}
	return m
}

func fillRect(buf []bool, n int, r rect, ox, scale float64) {
	px := func(v float64) int { return int(math.Round(v * scale)) }
	for y := px(glyphTop + r.y0); y < px(glyphTop+r.y1); y++ {
		for x := px(ox + r.x0); x < px(ox+r.x1); x++ {
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
