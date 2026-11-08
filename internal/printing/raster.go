package printing

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sync"

	"github.com/skip2/go-qrcode"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// DejaVu Sans Mono covers ₦, ₵, € and most scripts, under a permissive
// licence (see fonts/LICENSE-DejaVu.txt).
var (
	//go:embed fonts/DejaVuSansMono.ttf
	regularTTF []byte
	//go:embed fonts/DejaVuSansMono-Bold.ttf
	boldTTF []byte
)

type faces struct {
	regular, bold, bigBold font.Face
	adv                    int // dots per column, normal size
	lineH, bigLineH        int
	ascent, bigAscent      int
}

var (
	faceMu    sync.Mutex
	faceCache = map[Paper]*faces{}
	// renderMu serialises rendering: opentype faces keep internal buffers
	// and are not safe for concurrent use.
	renderMu sync.Mutex
)

func loadFaces(p Paper) (*faces, error) {
	faceMu.Lock()
	defer faceMu.Unlock()
	if f, ok := faceCache[p]; ok {
		return f, nil
	}
	reg, err := opentype.Parse(regularTTF)
	if err != nil {
		return nil, err
	}
	bold, err := opentype.Parse(boldTTF)
	if err != nil {
		return nil, err
	}
	adv := p.Dots / p.Cols // 12 dots for both 80mm/48 and 58mm/32

	// Find the point size whose advance width is exactly adv dots.
	size := sizeForAdvance(reg, adv)
	mk := func(f *opentype.Font, sz float64) (font.Face, error) {
		return opentype.NewFace(f, &opentype.FaceOptions{Size: sz, DPI: 72, Hinting: font.HintingFull})
	}
	fs := &faces{adv: adv}
	if fs.regular, err = mk(reg, size); err != nil {
		return nil, err
	}
	if fs.bold, err = mk(bold, size); err != nil {
		return nil, err
	}
	if fs.bigBold, err = mk(bold, sizeForAdvance(bold, adv*2)); err != nil {
		return nil, err
	}
	m := fs.regular.Metrics()
	fs.ascent = m.Ascent.Ceil()
	fs.lineH = (m.Ascent + m.Descent).Ceil() + 4
	bm := fs.bigBold.Metrics()
	fs.bigAscent = bm.Ascent.Ceil()
	fs.bigLineH = (bm.Ascent + bm.Descent).Ceil() + 6
	faceCache[p] = fs
	return fs, nil
}

func sizeForAdvance(f *opentype.Font, adv int) float64 {
	lo, hi := 4.0, 80.0
	for range 30 {
		mid := (lo + hi) / 2
		face, _ := opentype.NewFace(f, &opentype.FaceOptions{Size: mid, DPI: 72, Hinting: font.HintingNone})
		a, _ := face.GlyphAdvance('M')
		face.Close()
		if float64(a)/64 > float64(adv) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return lo
}

const (
	topPad    = 8
	bottomPad = 8
	ruleH     = 14
	spaceH    = 12
	qrSize    = 216 // dots; ~27mm, scans easily
)

// Render rasterises blocks onto a 1-bit image exactly p.Dots wide and as
// tall as the content.
func Render(blocks []Block, p Paper) (*image.Gray, error) {
	renderMu.Lock()
	defer renderMu.Unlock()
	f, err := loadFaces(p)
	if err != nil {
		return nil, err
	}
	// Pass 1: measure.
	h := topPad
	for _, b := range blocks {
		h += blockHeight(b, f)
	}
	h += bottomPad

	img := image.NewGray(image.Rect(0, 0, p.Dots, h))
	draw.Draw(img, img.Bounds(), image.White, image.Point{}, draw.Src)

	// Pass 2: draw.
	y := topPad
	for _, b := range blocks {
		bh := blockHeight(b, f)
		switch b.Kind {
		case KText:
			face, asc := f.regular, f.ascent
			if b.Bold {
				face = f.bold
			}
			if b.Big {
				face, asc = f.bigBold, f.bigAscent
			}
			d := &font.Drawer{Dst: img, Src: image.Black, Face: face, Dot: fixed.P(0, y+asc+2)}
			d.DrawString(b.Text)
		case KRule:
			dashed(img, y+bh/2, 1)
		case KDoubleRule:
			fillRow(img, y+bh/2-2, 2)
			fillRow(img, y+bh/2+2, 2)
		case KQR:
			q, err := qrcode.New(b.QR, qrcode.Medium)
			if err == nil {
				q.DisableBorder = true
				qi := q.Image(qrSize)
				x0 := (p.Dots - qrSize) / 2
				draw.Draw(img, image.Rect(x0, y+4, x0+qrSize, y+4+qrSize), qi, image.Point{}, draw.Src)
			}
		}
		y += bh
	}
	threshold(img)
	return img, nil
}

func blockHeight(b Block, f *faces) int {
	switch b.Kind {
	case KText:
		if b.Big {
			return f.bigLineH
		}
		return f.lineH
	case KRule, KDoubleRule:
		return ruleH
	case KSpace:
		return spaceH
	case KQR:
		return qrSize + 8
	}
	return 0
}

func fillRow(img *image.Gray, y, thick int) {
	for dy := range thick {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			img.SetGray(x, y+dy, color.Gray{0})
		}
	}
}

func dashed(img *image.Gray, y, thick int) {
	for dy := range thick {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			if (x/6)%2 == 0 {
				img.SetGray(x, y+dy, color.Gray{0})
			}
		}
	}
}

// threshold converts anti-aliased grey to pure black/white. A slightly
// generous cut-off gives thermal heads bolder, more legible strokes.
func threshold(img *image.Gray) {
	for i, v := range img.Pix {
		if v < 170 {
			img.Pix[i] = 0
		} else {
			img.Pix[i] = 255
		}
	}
}

// PNGDataURL encodes an image for display in the UI.
func PNGDataURL(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
