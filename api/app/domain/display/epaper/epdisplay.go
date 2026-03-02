package epaper

import (
	"fmt"
	"slices"
	"github.com/mikyk10/wisp/app/domain/model"
)

type EPaperDisplayModel string

const (
	WS4in0EPaperE  EPaperDisplayModel = "ws4in0e"  // 4.0 inch 6-Color (Shorter refresh time with E Ink Spectra 6)
	WS7in3EPaperF  EPaperDisplayModel = "ws7in3f"  // 7.3 inch 7-Color (Longer refresh time & narrow operating temperature)
	WS7in3EPaperE  EPaperDisplayModel = "ws7in3e"  // 7.3 inch E6 full color (Shorter refresh time with E Ink Spectra 6)
	WS13in3EPaperE EPaperDisplayModel = "ws13in3e" // 13.3 inch E6 full color (Shorter refresh time with E Ink Spectra 6)
	WS13in3EPaperK EPaperDisplayModel = "ws13in3k" // 13.3 inch 4 grayscale
)

// displayRegistry maps model name to factory function.
// To add a new model, only edit this map.
var displayRegistry = map[EPaperDisplayModel]func(model.CanonicalOrientation) DisplayMetadata{
	WS4in0EPaperE:  NewWS4in0E,
	WS7in3EPaperF:  NewWS7in3F,
	WS7in3EPaperE:  NewWS7in3E,
	WS13in3EPaperE: NewWS13in3E,
	WS13in3EPaperK: NewWS13in3K,
}

// IsValidModel reports whether m is a recognized display model.
func IsValidModel(m EPaperDisplayModel) bool {
	_, ok := displayRegistry[m]
	return ok
}

// ValidModels returns a sorted list of all recognized display model names.
func ValidModels() []string {
	models := make([]string, 0, len(displayRegistry))
	for m := range displayRegistry {
		models = append(models, string(m))
	}
	slices.Sort(models)
	return models
}

type ePaperDisplay struct {
	displayModel         EPaperDisplayModel
	width                int
	height               int
	nativeOrientation    model.CanonicalOrientation
	installedOrientation model.CanonicalOrientation
}

func (d *ePaperDisplay) ModelName() string {
	return string(d.displayModel)
}

func (d *ePaperDisplay) Width() int {
	return d.width
}

func (d *ePaperDisplay) Height() int {
	return d.height
}

func (d *ePaperDisplay) NativeOrientation() model.CanonicalOrientation {
	return d.nativeOrientation
}

func (d *ePaperDisplay) InstalledOrientation() model.CanonicalOrientation {
	return d.installedOrientation
}

func NewDisplay(m EPaperDisplayModel, orientation model.CanonicalOrientation) DisplayMetadata {
	factory, ok := displayRegistry[m]
	if !ok {
		panic(fmt.Sprintf("unsupported display model %q, check service.yaml", m))
	}
	return factory(orientation)
}
