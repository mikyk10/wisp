package catalog

import (
	"image"
	"image/color"
	"time"
	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/model"
)

func NewColorbarProvider(epd epaper.DisplayMetadata) ImageLocator {
	return &imageColorbarProvider{
		epd: epd,
	}
}

type imageColorbarProvider struct {
	epd epaper.DisplayMetadata
}

func (i *imageColorbarProvider) Resolve() (ImageLoader, error) {
	ilp := &imageLocalFilePointer{
		imageLoader: &imageLoader{},
		epd:         i.epd,
		path:        "generated",
	}

	width := i.epd.Width()
	height := i.epd.Height()

	// EBU 75% color bars (left to right)
	colors := []color.RGBA{
		{191, 191, 191, 255}, // White (75%)
		{191, 191, 0, 255},   // Yellow
		{0, 191, 191, 255},   // Cyan
		{0, 191, 0, 255},     // Green
		{191, 0, 191, 255},   // Magenta
		{191, 0, 0, 255},     // Red
		{0, 0, 191, 255},     // Blue
		{0, 0, 0, 255},       // Black
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	barWidth := width / len(colors)

	// Draw bars evenly in the horizontal direction
	for i, c := range colors {
		startX := i * barWidth
		endX := startX + barWidth
		if i == len(colors)-1 {
			endX = width // last bar absorbs any remainder
		}
		for x := startX; x < endX; x++ {
			for y := 0; y < height; y++ {
				img.Set(x, y, c)
			}
		}
	}

	meta := &model.ImgMeta{}
	meta.ImageSourcePath = "generated"
	meta.FileModifiedAt = time.Time{}

	ilp.img = img
	ilp.meta = meta

	return ilp, nil
}
