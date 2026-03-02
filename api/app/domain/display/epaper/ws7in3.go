package epaper

import (
	"image/color"
	"github.com/mikyk10/wisp/app/domain/display/epaper/wsdisplay"
	"github.com/mikyk10/wisp/app/domain/model"
)

func NewWS7in3F(orientation model.CanonicalOrientation) DisplayMetadata {
	dev := &wsDisplay{
		ePaperDisplay: ePaperDisplay{
			displayModel:         WS7in3EPaperF,
			width:                800,
			height:               480,
			nativeOrientation:    model.ImgCanonicalOrientationLandscape,
			installedOrientation: orientation,
		},
		paletteMap: map[int]color.Color{
			0: wsdisplay.Black,
			1: wsdisplay.White,
			2: wsdisplay.Green,
			3: wsdisplay.Blue,
			4: wsdisplay.Red,
			5: wsdisplay.Yellow,
			6: wsdisplay.Orange,
		},
	}

	return dev
}

func NewWS7in3E(orientation model.CanonicalOrientation) DisplayMetadata {
	dev := &wsDisplay{
		ePaperDisplay: ePaperDisplay{
			displayModel:         WS7in3EPaperE,
			width:                800,
			height:               480,
			nativeOrientation:    model.ImgCanonicalOrientationLandscape,
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
