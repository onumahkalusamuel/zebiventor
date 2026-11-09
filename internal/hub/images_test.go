package hub

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
)

func TestProductImages(t *testing.T) {
	ctx := context.Background()
	h, err := Open(ctx, Config{DataDir: t.TempDir(), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.Setup(ctx, SetupInput{StoreName: "S", AdminName: "A", AdminUsername: "admin", AdminPassword: "secret123"}); err != nil {
		t.Fatal(err)
	}
	id, err := h.SaveProduct(ctx, SystemSession, protocol.ProductInput{Product: protocol.Product{Name: "Efuru", Unit: "pc"},
		Variants: []protocol.VariantInput{{Variant: protocol.Variant{PriceMinor: 500000}}}})
	if err != nil {
		t.Fatal(err)
	}
	// A big transparent PNG becomes a small JPEG on white.
	src := image.NewNRGBA(image.Rect(0, 0, 1200, 1800))
	for y := range 1800 {
		for x := range 1200 {
			if x < 600 {
				src.Set(x, y, color.NRGBA{200, 30, 30, 255})
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, src)
	if _, err := h.SetProductImage(ctx, SystemSession, id, []byte("not a picture")); err == nil {
		t.Fatal("garbage accepted as a picture")
	}
	hash, err := h.SetProductImage(ctx, SystemSession, id, buf.Bytes())
	if err != nil || len(hash) != 32 {
		t.Fatalf("set image: %q %v", hash, err)
	}
	data, _ := h.Image(ctx, hash)
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 320 || b.Dy() != 480 {
		t.Fatalf("resized to %v, want 320x480", b)
	}
	if r, g, b, _ := img.At(250, 10).RGBA(); r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Fatalf("transparent area not white: %d %d %d", r>>8, g>>8, b>>8)
	}
	// The till learns the hash with the product.
	resp, _ := h.Pull(ctx, 0, 0)
	found := false
	for _, p := range resp.Products {
		found = found || (p.ID == id && p.ImageHash == hash)
	}
	if !found {
		t.Fatal("image hash not synced with the product")
	}
	// Removing it deletes the stored picture.
	if _, err := h.SetProductImage(ctx, SystemSession, id, nil); err != nil {
		t.Fatal(err)
	}
	if data, _ := h.Image(ctx, hash); data != nil {
		t.Fatal("picture kept after removal")
	}
}
