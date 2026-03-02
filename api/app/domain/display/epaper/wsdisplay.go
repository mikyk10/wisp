package epaper

import (
	"bytes"
	"image"
	"image/color"
	"github.com/mikyk10/wisp/app/domain/model"
)

type IndexPalette = map[int]color.Color

// DisplayMetadata describes the primary physical specifications of a display.
type DisplayMetadata interface {
	ModelName() string
	Width() int
	Height() int
	NativeOrientation() model.CanonicalOrientation
	InstalledOrientation() model.CanonicalOrientation
	Palette() IndexPalette
}

type wsDisplay struct {
	ePaperDisplay
	paletteMap map[int]color.Color // mapping from palette index to RGB color

}

func (d *wsDisplay) Palette() IndexPalette {
	return d.paletteMap
}

type Encoder interface {
	Encode(img image.Image) (*bytes.Buffer, error)
}
