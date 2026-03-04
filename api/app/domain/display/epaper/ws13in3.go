package epaper

import (
	"github.com/mikyk10/wisp/app/domain/display/epaper/wsdisplay"
	"github.com/mikyk10/wisp/app/domain/model"
	"image/color"
)

func NewWS13in3E(installedOrientation model.CanonicalOrientation) DisplayMetadata {
	dev := &wsDisplay{
		ePaperDisplay: ePaperDisplay{
			displayModel:         WS13in3EPaperE,
			width:                1200,
			height:               1600,
			nativeOrientation:    model.ImgCanonicalOrientationPortrait,
			installedOrientation: installedOrientation,
		},
		paletteMap: map[int]color.Color{
			0: wsdisplay.Black,
			1: wsdisplay.White,
			2: wsdisplay.Yellow,
			3: wsdisplay.Red,
			5: wsdisplay.Blue,
			6: wsdisplay.Green,
		},
	}
	return dev
}

// NewWS13in3K returns a new 13.3 inch e-Paper color display with a 960 × 680 pixels
// https://files.waveshare.com/wiki/13.3inch%20e-Paper%20HAT%2B/13.3inch_e-Paper_(E)_user_manual.pdf
func NewWS13in3K(installedOrientation model.CanonicalOrientation) DisplayMetadata {
	dev := &wsDisplay{
		ePaperDisplay: ePaperDisplay{
			displayModel:         WS13in3EPaperK,
			width:                960,
			height:               680,
			nativeOrientation:    model.ImgCanonicalOrientationLandscape,
			installedOrientation: installedOrientation,
		},
		paletteMap: map[int]color.Color{
			0: wsdisplay.Black,
			1: wsdisplay.DarkGray,
			2: wsdisplay.LightGray,
			3: wsdisplay.White,
		},
	}

	return dev
}
