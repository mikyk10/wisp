package catalog

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"strings"
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

func NewErrorMessageProviderFactory(epd epaper.DisplayMetadata, msg string, err error) ImageLocator {
	return &imageErrorMessageProvider{
		epd: epd,
		providerConfig: &config.ImageErrorMessageProviderConfig{
			Message: msg,
		},
		err: err,
	}
}

type imageErrorMessageProvider struct {
	epd            epaper.DisplayMetadata
	providerConfig *config.ImageErrorMessageProviderConfig
	err            error
}

func (ip *imageErrorMessageProvider) Resolve() (ImageLoader, error) {
	msg := ip.providerConfig.Message

	// Append a user-friendly description of the underlying error, unless it's a display-not-found error.
	if ip.err != nil {
		if _, ok := ip.err.(*DisplayNotFoundError); !ok {
			msg = ip.classifyError(ip.err)
		}
	}

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

	face = basicfont.Face7x13
	lines := strings.Split(msg, "\n")
	lineHeight := 16
	for i, line := range lines {
		d := &font.Drawer{
			Dst:  fgcanvas,
			Src:  image.NewUniform(wsdisplay.White),
			Face: face,
			Dot:  fixed.Point26_6{X: fixed.I(20), Y: fixed.I(iconY + size + 40 + i*lineHeight)},
		}
		d.DrawString(line)
	}

	return &imageLoader{
		img: fgcanvas,
		meta: &model.ImgMeta{
			ImageSourcePath:  "NOT_FOUND",
			ImageOrientation: ip.epd.NativeOrientation(),
		},
	}, nil
}

// classifyError analyzes an error and returns a user-friendly message for the error image.
func (ip *imageErrorMessageProvider) classifyError(err error) string {
	errStr := err.Error()

	// HTTP / network errors (external service unreachable)
	if strings.Contains(errStr, "http") ||
		strings.Contains(errStr, "url") ||
		strings.Contains(errStr, "unexpected content-type") {
		return "Failed to fetch image from external source.\n" + errStr
	}

	// Network-level errors (covers both DB and HTTP)
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "dial tcp") ||
		strings.Contains(errStr, "no such host") {
		return "Network error: target unreachable.\n" + errStr
	}

	// Database schema errors
	if strings.Contains(errStr, "no such table") ||
		strings.Contains(errStr, "no such column") ||
		strings.Contains(errStr, "constraint") {
		return "Database schema error.\nTables may be missing. Restart server to initialize database."
	}

	// Record not found (empty catalog)
	if strings.Contains(errStr, "record not found") {
		return "No images available.\nRun 'catalog scan' or 'catalog fetch' to index images."
	}

	return "Unexpected error:\n" + errStr
}
