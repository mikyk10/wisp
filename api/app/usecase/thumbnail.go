package usecase

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"

	"github.com/sunshineplan/imgconv"
)

const (
	thumbWidth     = 256
	thumbMaxHeight = 1024
	// thumb_jpg is a blob, a longblob or a bytea depending on the database, and
	// none of them imposes a ceiling worth defending against. This cap is ours:
	// thumbnails are read back in bulk by the catalogue listing, so an
	// unbounded one is paid for on every page.
	thumbMaxBytes = 60000
)

// encodeThumbnail resizes img and encodes it as a JPEG small enough for model.Image.ThumbJPG.
//
// Sizing by width alone leaves the height unbounded, so a tall enough source (a long
// screenshot, a vertical panorama) produces a thumbnail that overflows the cap. Cap the
// height for those, and step the quality down if the encoded result is still too large.
func encodeThumbnail(img image.Image) ([]byte, error) {
	opt := imgconv.ResizeOption{Width: thumbWidth}
	if b := img.Bounds(); b.Dx() > 0 && b.Dy()*thumbWidth/b.Dx() > thumbMaxHeight {
		opt = imgconv.ResizeOption{Height: thumbMaxHeight}
	}
	resized := imgconv.Resize(img, &opt)

	var buf bytes.Buffer
	for _, quality := range []int{jpeg.DefaultQuality, 60, 45} {
		buf.Reset()
		if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: quality}); err != nil {
			return nil, err
		}
		if buf.Len() <= thumbMaxBytes {
			return buf.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("thumbnail still %d bytes at lowest quality", buf.Len())
}
