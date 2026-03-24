package config

import (
	"fmt"
	"os"
	"time"
	"github.com/mikyk10/wisp/app/domain/finder"
	"github.com/mikyk10/wisp/app/domain/finder/fs"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/model/config/raw"

	"github.com/Code-Hex/synchro/iso8601"
	"github.com/adhocore/gronx"
	"github.com/caarlos0/env/v10"
	"github.com/mikyk10/wisp/app/domain/display/epaper"

	"gopkg.in/yaml.v2"
)

type defaultConfigLoader struct {
	finder finder.PathFinder
}

func NewDefaultConfigLoader() config.ConfigLoader {
	execName := "wisp" //filepath.Base(os.Args[0])
	return &defaultConfigLoader{
		finder: fs.NewConfigFilePathFinder(fmt.Sprintf("/etc/%s/", execName), fmt.Sprintf("$HOME/.%s", execName), "./config", "."),
	}
}

func NewTestConfigLoader() config.ConfigLoader {
	return &defaultConfigLoader{
		finder: fs.NewConfigFilePathFinder("testdata", "."),
	}
}

func (ldr *defaultConfigLoader) LoadConfig() (*config.GlobalConfig, *config.ServiceConfig, error) {

	conf, rawSvcConfig, err := ldr.loadRawConfig()
	if err != nil {
		return nil, nil, err
	}

	if err := validateGlobalConfig(conf); err != nil {
		return nil, nil, err
	}

	svcConfig := &config.ServiceConfig{}
	svcConfig.Catalog = make(map[string]*config.ImageProviderConfig)

	for _, v := range rawSvcConfig.Catalog {
		svcConfig.Catalog[v.Key] = parseCatalogEntry(v)
	}

	svcConfig.Displays = make(map[string]*config.DisplayConfig)
	for _, v := range rawSvcConfig.Displays {

		if !epaper.IsValidModel(epaper.EPaperDisplayModel(v.DisplayModel)) {
			return nil, nil, fmt.Errorf("display[%s]: unknown model %q", v.Key, v.DisplayModel)
		}

		disp := config.DisplayConfig{
			Name:            v.Name,
			Key:             v.Key,
			ApiVersion:      v.APIVersion,
			DisplayModel:    v.DisplayModel,
			Orientation:     config.NewDisplayOrientation(v.DisplayOrientation),
			Flip:            v.Flip,
			ShowTimestamp:   v.ShowTimestamp,
			Catalog:         make([]*config.AssociatedImageProviders, len(v.AssociatedCatalogEntry)),
			ImageProcessors: make([]*config.ImageProcessorConfig, len(v.ImageProcessors)),
			ColorReduction: config.ColorReduction{
				Type:     v.ColorReduction.Type,
				Size:     v.ColorReduction.Size,
				Strength: v.ColorReduction.Strength,
			},
			SleepDurationSeconds: v.SleepDurationSeconds,
		}

		if disp.SleepDurationSeconds == 0 {
			disp.SleepDurationSeconds = 86400
		}

		cropStrategy := config.CropStrategyCenter
		if v.Crop.Strategy == string(config.CropStrategyExifSubject) {
			cropStrategy = config.CropStrategyExifSubject
		}
		disp.Crop = config.CropConfig{Strategy: cropStrategy}

		gron := gronx.New()
		for i, cat := range v.AssociatedCatalogEntry {
			provConfig, ok := svcConfig.Catalog[cat.Key]
			if !ok {
				return nil, nil, fmt.Errorf("display[%s].catalog[%d]: unknown catalog key %q", v.Key, i, cat.Key)
			}
			if cat.TimeRange.Cron != "" && !gron.IsValid(cat.TimeRange.Cron) {
				return nil, nil, fmt.Errorf("display[%s].catalog[%d]: invalid cron expression %q", v.Key, i, cat.TimeRange.Cron)
			}
			disp.Catalog[i] = &config.AssociatedImageProviders{
				ProviderConfig: provConfig,
				TimeRange: config.CronConfig{
					Cron: cat.TimeRange.Cron,
				},
			}
		}

		for i, v := range v.ImageProcessors {
			disp.ImageProcessors[i] = &config.ImageProcessorConfig{
				Type: v.Type,
				Data: v.Properties,
			}
		}

		svcConfig.Displays[v.Key] = &disp
	}

	return conf, svcConfig, nil
}

func parseCatalogEntry(v raw.CatalogEntry) *config.ImageProviderConfig {
	switch v.Type {
	case config.ImageFileProviderType:
		cr := config.Criteria{}
		cr.Include.Path = v.FileConfig.Criteria.Include.Path
		cr.Exclude.Path = v.FileConfig.Criteria.Exclude.Path

		cr.Include.ExifTimeRange = make([]config.TimeRange, len(v.FileConfig.Criteria.Include.TimeRange))
		for i, r := range v.FileConfig.Criteria.Include.TimeRange {
			t, _ := iso8601.ParseDateTime(r.From)
			cr.Include.ExifTimeRange[i].From = t
			t, _ = iso8601.ParseDateTime(r.To)
			cr.Include.ExifTimeRange[i].To = t
			d, _ := time.ParseDuration(r.Last)
			cr.Include.ExifTimeRange[i].Last = d
		}

		cr.Exclude.ExifTimeRange = make([]config.TimeRange, len(v.FileConfig.Criteria.Exclude.TimeRange))
		for i, r := range v.FileConfig.Criteria.Exclude.TimeRange {
			t, _ := iso8601.ParseDateTime(r.From)
			cr.Exclude.ExifTimeRange[i].From = t
			t, _ = iso8601.ParseDateTime(r.To)
			cr.Exclude.ExifTimeRange[i].To = t
			d, _ := time.ParseDuration(r.Last)
			cr.Exclude.ExifTimeRange[i].Last = d
		}

		return &config.ImageProviderConfig{
			Key: v.Key,
			Config: config.ImageFileProviderConfig{
				Criteria: cr,
				SrcPath:  v.FileConfig.SrcPath,
				Hooks: config.FileHooks{
					OnNewFile: v.FileConfig.Hooks.OnNewFile,
				},
			},
		}

	case config.ImageHTTPProviderType:
		return &config.ImageProviderConfig{
			Key:    v.Key,
			Config: config.ImageHTTPProviderConfig{URL: v.HTTPConfig.URL},
		}

	case config.ImagePlaywrightProviderType:
		return &config.ImageProviderConfig{
			Key: v.Key,
			Config: config.ImagePlaywrightProviderConfig{
				URL:    v.PlaywrightConfig.URL,
				Server: v.PlaywrightConfig.Server,
			},
		}

	case config.ImageLuaProviderType:
		return &config.ImageProviderConfig{
			Key:    v.Key,
			Config: config.ImageLuaProviderConfig{Script: v.LuaConfig.Script},
		}

	case config.ImageColorbarProviderType:
		return &config.ImageProviderConfig{
			Key:    v.Key,
			Config: config.ImageColorbarProviderConfig{},
		}

	case config.ImageGenerateProviderType:
		stages := make([]config.StageConfig, len(v.GenerateConfig.Pipeline.Stages))
		for i, s := range v.GenerateConfig.Pipeline.Stages {
			stages[i] = config.StageConfig{
				Name:       s.Name,
				Output:     s.Output,
				Prompt:     s.Prompt,
				ImageInput: s.ImageInput,
			}
		}
		return &config.ImageProviderConfig{
			Key: v.Key,
			Config: config.ImageGenerateProviderConfig{
				CacheDepth:    v.GenerateConfig.CacheDepth,
				EvictCount:    v.GenerateConfig.EvictCount,
				SourceCatalog: v.GenerateConfig.SourceCatalog,
				Pipeline: config.PipelineConfig{
					Stages: stages,
				},
			},
		}
	}

	return nil
}

func validateGlobalConfig(conf *config.GlobalConfig) error {
	switch conf.Database.Driver {
	case "sqlite", "mysql":
		// valid
	default:
		return fmt.Errorf("invalid database driver: %q (must be sqlite or mysql)", conf.Database.Driver)
	}
	return nil
}

func (ldr *defaultConfigLoader) loadRawConfig() (*config.GlobalConfig, *raw.ServiceConfig, error) {
	configPath := ldr.finder.Find("config.yaml")
	svcConfPath := ldr.finder.Find("service.yaml")

	b, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, err
	}
	var conf config.GlobalConfig

	if err := yaml.Unmarshal(b, &conf); err != nil {
		return nil, nil, err
	}

	c, err := os.ReadFile(svcConfPath)
	if err != nil {
		return nil, nil, err
	}

	var rawServiceConfig raw.ServiceConfig
	if err := yaml.Unmarshal(c, &rawServiceConfig); err != nil {
		return nil, nil, err
	}

	// load environment variables; struct fields corresponding to environment variables will be overwritten
	if err := env.Parse(&conf); err != nil {
		return nil, nil, err
	}

	return &conf, &rawServiceConfig, nil
}
