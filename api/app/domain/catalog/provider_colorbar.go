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

	// SMPTE color bars (top to bottom: white, yellow, cyan, green, magenta, red, blue, black)
	colors := []color.RGBA{
		{255, 255, 255, 255}, // White
		{255, 255, 0, 255},   // Yellow
		{0, 255, 255, 255},   // Cyan
		{0, 255, 0, 255},     // Green
		{255, 0, 255, 255},   // Magenta
		{255, 0, 0, 255},     // Red
		{0, 0, 255, 255},     // Blue
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
