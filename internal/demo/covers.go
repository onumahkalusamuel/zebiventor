package demo

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/onumahkalusamuel/zebiventor/internal/documents"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// FetchCovers downloads each book's cover (medium size) into cache, once.
// Open Library serves covers by cover id without rate limits; we still go
// one at a time.
func FetchCovers(ctx context.Context, cache string, logf func(string, ...any)) error {
	books, err := Books()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	var got, missing int
	for i, b := range books {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		path := filepath.Join(cache, b.ISBN+".jpg")
		if _, err := os.Stat(path); err == nil || b.CoverID == 0 {
			continue
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg", b.CoverID), nil)
		req.Header.Set("User-Agent", userAgent)
		resp, err := client.Do(req)
		if err != nil {
			logf("cover %s: %v", b.ISBN, err)
			continue
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK || len(data) < 2000 {
			missing++
			continue
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
		got++
		if (i+1)%50 == 0 {
			logf("covers: %d/%d", i+1, len(books))
		}
		time.Sleep(120 * time.Millisecond)
	}
	logf("covers downloaded: %d (no cover for %d; those get a drawn cover)", got, missing)
	return nil
}

// Cover returns a book's picture: the downloaded cover, or a drawn one.
func Cover(cache string, b Book) []byte {
	if data, err := os.ReadFile(filepath.Join(cache, b.ISBN+".jpg")); err == nil && len(data) > 2000 {
		return data
	}
	return DrawCover(b.Title, b.Author)
}

var coverInks = []color.RGBA{
	{31, 78, 95, 255}, {91, 58, 41, 255}, {47, 72, 88, 255}, {74, 63, 107, 255},
	{45, 90, 61, 255}, {122, 46, 58, 255}, {102, 92, 30, 255}, {30, 79, 122, 255},
}

// DrawCover makes a simple typographic cover: title and author on a
// colour picked from the title.
func DrawCover(title, author string) []byte {
	const w, h = 320, 480
	hsh := fnv.New32a()
	hsh.Write([]byte(title))
	ink := coverInks[hsh.Sum32()%uint32(len(coverInks))]
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{ink}, image.Point{}, draw.Src)
	band := color.RGBA{255, 255, 255, 40}
	draw.Draw(img, image.Rect(0, 300, w, 304), &image.Uniform{band}, image.Point{}, draw.Over)

	bold, _ := opentype.Parse(documents.FontBold())
	reg, _ := opentype.Parse(documents.FontRegular())
	face := func(f *opentype.Font, size float64) font.Face {
		fc, _ := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		return fc
	}
	paper := image.NewUniform(color.RGBA{248, 244, 236, 255})
	write := func(fc font.Face, text string, y int) int {
		d := &font.Drawer{Dst: img, Src: paper, Face: fc}
		line := ""
		lh := fc.Metrics().Height.Ceil()
		flush := func() {
			d.Dot = fixed.P(28, y)
			d.DrawString(line)
			y += lh
			line = ""
		}
		for _, word := range strings.Fields(text) {
			try := strings.TrimSpace(line + " " + word)
			if d.MeasureString(try).Ceil() > w-56 && line != "" {
				flush()
				line = word
			} else {
				line = try
			}
		}
		if line != "" {
			flush()
		}
		return y
	}
	size := 34.0
	if len(title) > 40 {
		size = 26
	}
	write(face(bold, size), title, 90)
	write(face(reg, 20), author, 350)
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	return buf.Bytes()
}
