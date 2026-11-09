package hub

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png" // decoders for uploads
	"io"

	"github.com/onumahkalusamuel/zebiventor/internal/ids"
	"github.com/onumahkalusamuel/zebiventor/internal/protocol"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Product images live in the hub database (so backups and exports carry
// them), resized to at most maxImageSide and stored as JPEG. Products carry
// the image's hash; tills fetch an image by hash when they first show it
// and keep it, so a changed picture is simply a new hash.

const (
	maxImageSide  = 480
	maxImageBytes = 8 << 20
)

// NormaliseImage decodes a JPEG, PNG or WebP picture, shrinks it to fit
// maxImageSide and re-encodes it as JPEG.
func NormaliseImage(data []byte) ([]byte, error) {
	if len(data) > maxImageBytes {
		return nil, invalid("the picture is too large (8 MB at most)")
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, invalid("that file isn't a picture the app can read (use JPEG, PNG or WebP)")
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil, invalid("the picture is empty")
	}
	if w > maxImageSide || h > maxImageSide {
		if w >= h {
			h, w = h*maxImageSide/w, maxImageSide
		} else {
			w, h = w*maxImageSide/h, maxImageSide
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src) // transparent PNGs on white
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// SetProductImage stores (or, with nil data, removes) a product's picture.
// It returns the new image hash.
func (h *Hub) SetProductImage(ctx context.Context, by *Session, productID string, data []byte) (string, error) {
	if !by.Can(catalogRoles...) {
		return "", ErrForbidden
	}
	var hash string
	if data != nil {
		img, err := NormaliseImage(data)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(img)
		hash = hex.EncodeToString(sum[:16])
		data = img
	}
	err := h.write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		now := ids.NowMs()
		res, err := tx.ExecContext(ctx, "UPDATE products SET image_hash=?, updated_at=? WHERE id=?", hash, now, productID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return invalid("product not found")
		}
		if hash != "" {
			if _, err := tx.ExecContext(ctx, `INSERT INTO images(hash,data,created_at) VALUES(?,?,?) ON CONFLICT(hash) DO NOTHING`, hash, data, now); err != nil {
				return err
			}
		}
		// Pictures no product uses any more.
		if _, err := tx.ExecContext(ctx, "DELETE FROM images WHERE hash NOT IN (SELECT image_hash FROM products WHERE image_hash<>'')"); err != nil {
			return err
		}
		audit(ctx, tx, by.UserID, "", "product.image", "product", productID, map[string]any{"image": hash})
		return recordChange(ctx, tx, protocol.EntityProduct, productID)
	})
	return hash, err
}

// Image returns a stored picture by hash.
func (h *Hub) Image(ctx context.Context, hash string) ([]byte, error) {
	var data []byte
	err := h.db.R().QueryRowContext(ctx, "SELECT data FROM images WHERE hash=?", hash).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return data, err
}

func readAllLimit(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxImageBytes+1))
	if err == nil && len(b) > maxImageBytes {
		return nil, invalid("the picture is too large (8 MB at most)")
	}
	return b, err
}
