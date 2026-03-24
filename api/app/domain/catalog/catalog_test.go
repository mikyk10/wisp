package catalog

import (
	"testing"
)

/*
func TestConfigLoad(t *testing.T) {
	_, svcConf, _ := NewTestConfigLoader().LoadConfig()

	picker := NewFirstImageProviderConfig(
		&config.AssociatedImageProviders[any]{ProviderConfig: svcConf.Catalog["album-01"], TimeRange: config.CronConfig{Cron: "* * 13-14 * * *"}},
		&config.AssociatedImageProviders[any]{ProviderConfig: svcConf.Catalog["album-02"], TimeRange: config.CronConfig{Cron: "* * 1-2 * * *"}},
		&config.AssociatedImageProviders[any]{ProviderConfig: svcConf.Catalog["album-http"], TimeRange: config.CronConfig{Cron: "* * 14 * * *"}},
	)

	aaa := picker()
	spew.Dump(aaa)
	catalog := NewImageProviderFactory(aaa)
	spew.Dump(catalog())
}
*/

func TestConfigLoad(t *testing.T) {
	/*
		_, svcConf, _ := config.NewTestConfigLoader().LoadConfig()

		displayInUse := svcConf.Displays["00:00:00:00:00:00"]

		// Retrieve the image catalog for the display.

		// Resolve ImageProvider from the image catalog.
		imgProvider := imgCatalog.PickImageProvider(time.Now())

		//slices.Collect(svcConf.Displays)
		//aaaa := slices.Collect(maps.Values(svcConf.Displays))

		// Pass the target display and orientation to start processing.
		epDisplayMeta := epaper.NewDisplay(epaper.EPaperDisplayModel(displayInUse.DisplayModel), model.CanonicalOrientation(displayInUse.Orientation))
		if epDisplayMeta == nil {
			// yaml validation required
			panic("config problem") //TODO: error handling
		}

		imPtr := imgProvider.Resolve(epDisplayMeta)

		// Process according to provider and display settings.
		imseq := improc.NewSequencer()
		imseq.Push(crop.NewImageCropper(epDisplayMeta, config.CropStrategyCenter))
		//	imseq.Push(saturation.NewSaturationFactory()())
		//	imseq.Push(hue.NewImageHueFactory()())
		//	imseq.Push(color_reduction.NewImageColorReductionFactory(epDisplayMeta)())
		imseq.Push(timestamp.NewTimstamp())

		meta := imPtr.GetMeta()
		meta.ExifDateTime = time.Now()
		resultImg, _ := imseq.Apply(context.Background(), imPtr.GetImage(), meta)

		// Encode.

		f, _ := os.OpenFile("/tmp/test.png", os.O_CREATE|os.O_WRONLY, 0644)
		if err := png.Encode(f, resultImg); err != nil {
			f.Close()
			log.Fatal(err)
		}

		if err := f.Close(); err != nil {
			log.Fatal(err)
		}*/
}
