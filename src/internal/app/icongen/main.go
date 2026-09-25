// Command icongen writes icon_bg_<color>_<n>.png (n = 2..9), each base
// icon_bg_<color>.png with its instance number in a badge at the lower right.
//
//	go run ./icongen [dir]   (dir defaults to icons)
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var baseName = regexp.MustCompile(`^icon_bg_[a-z]+\.png$`)

var (
	badgeRing = color.NRGBA{0xff, 0xff, 0xff, 0xff}
	badgeFill = color.NRGBA{0x20, 0x20, 0x20, 0xff}
)

func main() {
	dir := "icons"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	paths, err := filepath.Glob(filepath.Join(dir, "icon_bg_*.png"))
	if err != nil {
		log.Fatal(err)
	}
	ttf, err := opentype.Parse(gobold.TTF)
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
		face, err := opentype.NewFace(ttf, &opentype.FaceOptions{
			Size:    float64(base.Bounds().Dx()) * 0.27,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err != nil {
			log.Fatal(err)
		}
		for n := 2; n <= 9; n++ {
			img := image.NewNRGBA(base.Bounds())
			draw.Draw(img, img.Bounds(), base, base.Bounds().Min, draw.Src)
			drawBadge(img, face, strconv.Itoa(n))
			out := path[:len(path)-len(".png")] + fmt.Sprintf("_%d.png", n)
			if err := writePNG(out, img); err != nil {
				log.Fatal(err)
			}
		}
	}
}

// drawBadge paints a ringed disc in the lower-right corner with label on it.
func drawBadge(img *image.NRGBA, face font.Face, label string) {
	b := img.Bounds()
	size := float64(b.Dx())
	r := size * 0.2
	c := size - r - size/64
	fill(img, disc(b, c, c, r), badgeRing)
	fill(img, disc(b, c, c, r-size/32), badgeFill)

	bounds, _ := font.BoundString(face, label)
	center := fixed.Int26_6(c * 64)
	d := font.Drawer{
		Dst:  img,
		Src:  image.White,
		Face: face,
		Dot: fixed.Point26_6{
			X: center - (bounds.Min.X+bounds.Max.X)/2,
			Y: center - (bounds.Min.Y+bounds.Max.Y)/2,
		},
	}
	d.DrawString(label)
}

// disc returns an anti-aliased coverage mask of a circle, 4x4 supersampled.
func disc(b image.Rectangle, cx, cy, r float64) *image.Alpha {
	const ss = 4
	m := image.NewAlpha(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			hits := 0
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					dx := float64(x) + (float64(sx)+0.5)/ss - cx
					dy := float64(y) + (float64(sy)+0.5)/ss - cy
					if dx*dx+dy*dy <= r*r {
						hits++
					}
				}
			}
			m.SetAlpha(x, y, color.Alpha{uint8(hits * 255 / (ss * ss))})
		}
	}
	return m
}

func fill(img *image.NRGBA, mask *image.Alpha, c color.NRGBA) {
	draw.DrawMask(img, img.Bounds(), image.NewUniform(c), image.Point{}, mask, mask.Bounds().Min, draw.Over)
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
