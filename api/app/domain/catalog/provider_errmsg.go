package catalog

import (
	"bytes"
	"image"
	"image/draw"
	"wspf/app/domain/display/epaper"
	"wspf/app/domain/display/epaper/wsdisplay"
	"wspf/app/domain/improc"
	"wspf/app/domain/model"
	"wspf/app/domain/model/config"

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
	draw.Draw(fgcanvas, fgcanvas.Bounds(), &image.Uniform{wsdisplay.White}, image.Point{}, draw.Src)

	fgcBounds := fgcanvas.Bounds()

	confusedPFIcon, _, _ := image.Decode(bytes.NewReader(errorPng))

	size := int(float64(fgcBounds.Max.X) * 0.1)
	confusedPFIcon = imgconv.Resize(confusedPFIcon, &imgconv.ResizeOption{Width: size, Height: size})

	xxx1 := (fgcBounds.Max.X / 2) - (size / 2)
	yyy1 := (fgcBounds.Max.Y / 2) - (size / 2)

	xxx2 := (fgcBounds.Max.X / 2) + (size / 2)
	yyy2 := (fgcBounds.Max.Y / 2) + (size / 2)

	draw.Draw(fgcanvas, image.Rectangle{image.Point{xxx1, yyy1}, image.Point{xxx2, yyy2}}, confusedPFIcon, image.Point{0, 0}, draw.Over)

	unkempt, _ := truetype.Parse(improc.Unkempt)

	face := truetype.NewFace(unkempt, &truetype.Options{
		Size: 64,
	})

	// Set up the struct for drawing.
	d := &font.Drawer{
		Dst:  fgcanvas,
		Src:  image.NewUniform(wsdisplay.Black),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(20), Y: fixed.I(70)},
	}
	d.DrawString("Ooops!")

	face = truetype.NewFace(unkempt, &truetype.Options{
		Size: 32,
	})
	d = &font.Drawer{
		Dst:  fgcanvas,
		Src:  image.NewUniform(wsdisplay.Black),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(30), Y: fixed.I(125)},
	}
	d.DrawString("The server's in trouble!!")

	face = basicfont.Face7x13
	d = &font.Drawer{
		Dst:  fgcanvas,
		Src:  image.NewUniform(wsdisplay.Black),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(30), Y: fixed.I(140)},
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
