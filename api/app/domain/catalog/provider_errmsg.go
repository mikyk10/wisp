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

	// Classify error details, unless it's a display-not-found error
	// TODO: improve error type detection using errors.As or errors.Is
	if ip.err != nil {
		if _, ok := ip.err.(*DisplayNotFoundError); !ok {
			msg = ip.classifyDatabaseError(ip.err)
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

// classifyDatabaseError analyzes a database error and returns a user-friendly message
// TODO: replace string matching with proper error type detection (e.g., errors.As for driver-specific errors)
func (ip *imageErrorMessageProvider) classifyDatabaseError(err error) string {
	errStr := err.Error()

	// Connection errors (database unavailable, network issue, auth failure)
	if strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "dial") {
		return "Database connection failed.\nCheck server availability and credentials."
	}

	// Query errors (schema mismatch, syntax error, constraint violation)
	if strings.Contains(errStr, "syntax") ||
		strings.Contains(errStr, "column") ||
		strings.Contains(errStr, "unknown") ||
		strings.Contains(errStr, "constraint") ||
		strings.Contains(errStr, "no such table") {
		return "Database schema error.\nTables may be missing. Restart server to initialize database."
	}

	// Generic database error with actual error message
	return "Database error:\n" + errStr
}
