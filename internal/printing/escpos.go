package printing

import (
	"bytes"
	"image"
	"strings"
)

// ESC/POS command bytes.
var (
	cmdInit        = []byte{0x1B, 0x40}                   // ESC @
	cmdAlignLeft   = []byte{0x1B, 0x61, 0x00}             // ESC a 0
	cmdAlignCenter = []byte{0x1B, 0x61, 0x01}             // ESC a 1
	cmdBoldOn      = []byte{0x1B, 0x45, 0x01}             // ESC E 1
	cmdBoldOff     = []byte{0x1B, 0x45, 0x00}             // ESC E 0
	cmdSizeNormal  = []byte{0x1D, 0x21, 0x00}             // GS ! 0
	cmdSizeDouble  = []byte{0x1D, 0x21, 0x11}             // GS ! 0x11 (2x width, 2x height)
	cmdDrawerKick  = []byte{0x1B, 0x70, 0x00, 0x19, 0xFA} // ESC p 0 25 250 (pin 2)
	cmdLineSpacing = []byte{0x1B, 0x33, 0x00}             // ESC 3 0: no gap between raster bands
	cmdLineDefault = []byte{0x1B, 0x32}                   // ESC 2
)

// JobOptions controls the job wrapper around the content.
type JobOptions struct {
	OpenDrawer bool
	Cut        bool
	FeedLines  int // lines fed before cutting so the last line clears the cutter
}

// DefaultJob is suitable for most 80mm printers.
var DefaultJob = JobOptions{Cut: true, FeedLines: 4}

func cmdFeedCut(feed int) []byte {
	if feed < 0 {
		feed = 0
	}
	// GS V 66 n: feed n units then partial cut — one command, no wasted paper.
	return []byte{0x1D, 0x56, 0x42, byte(min(feed*24, 255))}
}

// bandRows is the height of each GS v 0 chunk. Small bands keep cheap
// printers (with small receive buffers) happy on very long receipts.
const bandRows = 128

// EncodeRaster converts a rendered receipt into an ESC/POS job.
func EncodeRaster(img *image.Gray, o JobOptions) []byte {
	var b bytes.Buffer
	b.Write(cmdInit)
	if o.OpenDrawer {
		b.Write(cmdDrawerKick)
	}
	b.Write(cmdLineSpacing)
	b.Write(cmdAlignLeft)

	w := img.Rect.Dx()
	h := img.Rect.Dy()
	xBytes := (w + 7) / 8
	for y0 := 0; y0 < h; y0 += bandRows {
		rows := min(bandRows, h-y0)
		// GS v 0 m xL xH yL yH d1...dk
		b.Write([]byte{0x1D, 0x76, 0x30, 0x00, byte(xBytes), byte(xBytes >> 8), byte(rows), byte(rows >> 8)})
		for y := y0; y < y0+rows; y++ {
			row := img.Pix[(y)*img.Stride : (y)*img.Stride+w]
			for xb := 0; xb < xBytes; xb++ {
				var v byte
				for bit := range 8 {
					x := xb*8 + bit
					if x < w && row[x] < 128 { // black dot
						v |= 0x80 >> bit
					}
				}
				b.WriteByte(v)
			}
		}
	}
	b.Write(cmdLineDefault)
	if o.Cut {
		b.Write(cmdFeedCut(o.FeedLines))
	} else {
		b.Write([]byte{0x1B, 0x64, byte(o.FeedLines)}) // ESC d n
	}
	return b.Bytes()
}

// EncodeText produces a text-mode ESC/POS job. Only ASCII is sent; other
// characters are transliterated (₦ -> N) because printer code pages rarely
// contain them. Use raster mode for full fidelity.
func EncodeText(blocks []Block, cols int, o JobOptions) []byte {
	var b bytes.Buffer
	b.Write(cmdInit)
	if o.OpenDrawer {
		b.Write(cmdDrawerKick)
	}
	b.Write(cmdAlignLeft)
	for _, bl := range blocks {
		switch bl.Kind {
		case KText:
			if bl.Bold {
				b.Write(cmdBoldOn)
			}
			if bl.Big {
				b.Write(cmdSizeDouble)
			}
			b.WriteString(asciiOnly(bl.Text))
			b.WriteByte('\n')
			if bl.Big {
				b.Write(cmdSizeNormal)
			}
			if bl.Bold {
				b.Write(cmdBoldOff)
			}
		case KRule:
			b.WriteString(strings.Repeat("-", cols) + "\n")
		case KDoubleRule:
			b.WriteString(strings.Repeat("=", cols) + "\n")
		case KSpace:
			b.WriteByte('\n')
		case KQR:
			b.Write(cmdAlignCenter)
			b.Write(qrCommands(bl.QR))
			b.Write(cmdAlignLeft)
		}
	}
	if o.Cut {
		b.Write(cmdFeedCut(o.FeedLines))
	} else {
		b.Write([]byte{0x1B, 0x64, byte(o.FeedLines)})
	}
	return b.Bytes()
}

// qrCommands uses the printer's native QR generator (GS ( k).
func qrCommands(data string) []byte {
	var b bytes.Buffer
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x04, 0x00, 0x31, 0x41, 0x32, 0x00}) // model 2
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x43, 0x06})       // module size 6
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x45, 0x31})       // error correction M
	n := len(data) + 3
	b.Write([]byte{0x1D, 0x28, 0x6B, byte(n), byte(n >> 8), 0x31, 0x50, 0x30})
	b.WriteString(data)
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x51, 0x30}) // print
	b.WriteByte('\n')
	return b.Bytes()
}

// Single-character substitutes keep column alignment intact.
var translit = strings.NewReplacer("₦", "N", "₵", "C", "€", "E", "£", "L", "’", "'", "‘", "'", "“", "\"", "”", "\"", "–", "-", "—", "-", "…", ".")

func asciiOnly(s string) string {
	s = translit.Replace(s)
	var b strings.Builder
	for _, r := range s {
		if r < 0x20 || r > 0x7E {
			b.WriteByte('?')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// DrawerKick returns a job that only opens the cash drawer.
func DrawerKick() []byte {
	return append(append([]byte{}, cmdInit...), cmdDrawerKick...)
}
