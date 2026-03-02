package epaper

import (
	"image/color"
	"github.com/mikyk10/wisp/app/domain/display/epaper/wsdisplay"
	"github.com/mikyk10/wisp/app/domain/model"
)

func NewWS4in0E(orientation model.CanonicalOrientation) DisplayMetadata {
	dev := &wsDisplay{
		ePaperDisplay: ePaperDisplay{
			displayModel:         WS4in0EPaperE,
			width:                400,
			height:               600,
			nativeOrientation:    model.ImgCanonicalOrientationPortrait,
			installedOrientation: orientation,
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
