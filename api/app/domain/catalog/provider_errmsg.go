package catalog

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/display/epaper/wsdisplay"
	"github.com/mikyk10/wisp/app/domain/improc"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"

	"github.com/golang/freetype/truetype"
	"github.com/sunshineplan/imgconv"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	_ "embed"
)

//go:embed error-icon.png
var errorPng []byte

func NewErrorMessageProviderFactory(epd epaper.DisplayMetadata, msg string) ImageLocator {
	return &imageErrorMessageProvider{
		epd: epd,
		providerConfig: &config.ImageErrorMessageProviderConfig{
			Message: msg,
		},
	}
}

type imageErrorMessageProvider struct {
	epd            epaper.DisplayMetadata
	providerConfig *config.ImageErrorMessageProviderConfig
}

func (ip *imageErrorMessageProvider) Resolve() (ImageLoader, error) {

	width := ip.epd.Width()
	height := ip.epd.Height()

	if ip.epd.NativeOrientation() != ip.epd.InstalledOrientation() {
		width, height = height, width
	}

	fgcanvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(fgcanvas, fgcanvas.Bounds(), &image.Uniform{color.RGBA{0x00, 0x00, 0x00, 0xff}}, image.Point{}, draw.Src)

	fgcBounds := fgcanvas.Bounds()

	confusedPFIcon, _, _ := image.Decode(bytes.NewReader(errorPng))

	size := int(float64(fgcBounds.Max.X) * 0.08)
	confusedPFIcon = imgconv.Resize(confusedPFIcon, &imgconv.ResizeOption{Width: size, Height: size})

	// Position icon at top-left
	iconX := 20
	iconY := 20

	draw.Draw(fgcanvas, image.Rectangle{image.Point{iconX, iconY}, image.Point{iconX + size, iconY + size}}, confusedPFIcon, image.Point{0, 0}, draw.Over)

	unkempt, _ := truetype.Parse(improc.Unkempt)

	face := truetype.NewFace(unkempt, &truetype.Options{
		Size:            48,
		Hinting:         font.HintingFull,
		SubPixelsX:      0, // Disable sub-pixel rendering (anti-aliasing)
		SubPixelsY:      0,
	})

	// Position "Ooops!" to the right of the icon
	d := &font.Drawer{
		Dst:  fgcanvas,
		Src:  image.NewUniform(wsdisplay.White),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(iconX + size + 20), Y: fixed.I(iconY + size - 10)},
	}
	d.DrawString("Ooops!")

	face = truetype.NewFace(unkempt, &truetype.Options{
		Size:            28,
		Hinting:         font.HintingFull,
		SubPixelsX:      0, // Disable sub-pixel rendering (anti-aliasing)
		SubPixelsY:      0,
	})
	d = &font.Drawer{
		Dst:  fgcanvas,
		Src:  image.NewUniform(wsdisplay.White),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(20), Y: fixed.I(iconY + size + 40)},
	}
	d.DrawString("The server's in trouble!!")

	face = basicfont.Face7x13
	d = &font.Drawer{
		Dst:  fgcanvas,
		Src:  image.NewUniform(wsdisplay.White),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(20), Y: fixed.I(iconY + size + 75)},
	}

	d.DrawString(ip.providerConfig.Message)

	return &imageLoader{
		img: fgcanvas,
		meta: &model.ImgMeta{
			ImageSourcePath:  "NOT_FOUND",
			ImageOrientation: ip.epd.NativeOrientation(),
			// Available color indices differ by display (probably fine, but there are compatibility concerns).
			// White and black are available on almost every display, so as long as the mapping logic is correct...
			SkipColorReduction: true,
		},
	}, nil
}
