package cmd

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/usecase"

	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

func NewAlbumScanCommand(c *dig.Container) *cobra.Command {

	var catUc usecase.CatalogUsecase
	if err := c.Invoke(func(sConf *config.ServiceConfig, uc usecase.CatalogUsecase) {
		catUc = uc
	}); err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan albums for new images",
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				})))
			}
			workers, _ := cmd.Flags().GetInt("workers")
			return catUc.Scan(workers)
		},
	}
	cmd.Flags().BoolP("verbose", "v", false, "Show all included, excluded, and skipped files")
	cmd.Flags().IntP("workers", "w", 0, "Number of parallel image-processing goroutines (0 = auto: min(GOMAXPROCS,4), or WISP_SCAN_CONCURRENCY)")

	return cmd
}

func NewCatalogListCommand(c *dig.Container) *cobra.Command {

	var serviceConfig *config.ServiceConfig
	if err := c.Invoke(func(sConf *config.ServiceConfig) {
		serviceConfig = sConf
	}); err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}

	return &cobra.Command{
		Use:   "list",
		Short: "List all catalogs",
		RunE: func(cmd *cobra.Command, args []string) error {
			catalogs := slices.Collect(maps.Values(serviceConfig.Catalog))
			sort.Slice(catalogs, func(i, j int) bool {
				return strings.Compare(catalogs[i].Key, catalogs[j].Key) < 0
			})

			for _, v := range catalogs {
				switch prv := v.Config.(type) {
				case config.ImageFileProviderConfig:
					fmt.Printf("%s : FileProvider(%s)\n", v.Key, prv.SrcPath)
				case config.ImageHTTPProviderConfig:
					fmt.Printf("%s : HTTPProvider\n", v.Key)
				case config.ImagePlaywrightProviderConfig:
					fmt.Printf("%s : PlayWrightProvider\n", v.Key)
				case config.ImageLuaProviderConfig:
					fmt.Printf("%s : LuaProvider\n", v.Key)
				}
			}

			return nil
		}}
}

func NewCatalogListImagesCommand(c *dig.Container) *cobra.Command {
	var serviceConfig *config.ServiceConfig
	var catUc usecase.CatalogUsecase
	if err := c.Invoke(func(sConf *config.ServiceConfig, uc usecase.CatalogUsecase) {
		serviceConfig = sConf
		catUc = uc
	}); err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}

	cmd := &cobra.Command{
		Use:   "lsimg",
		Short: "List all images in catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			catlName, _ := cmd.Flags().GetString("catalog")
			if catlName == "" {
				return errors.New("error: please specify catalog name")
			}

			catl, ok := serviceConfig.Catalog[catlName]
			if !ok {
				return errors.New("error: catalog not found")
			}

			_, ok = catl.Config.(config.ImageFileProviderConfig)
			if !ok {
				return errors.New("error: Only file catalog type can list images")
			}

			return catUc.ListImages(catlName, nil, func(img *model.Image) error {
				takenAt := "-"
				if img.TakenAt.Valid {
					takenAt = img.TakenAt.Time.Format("2006-01-02")
				}
				fmt.Printf("%d\t%s\t%s\n", img.ID, img.Src, takenAt)
				return nil
			})
		}}
	cmd.Flags().StringP("catalog", "c", "", "Catalog name")

	return cmd
}

func NewAlbumCleanupCommand(c *dig.Container) *cobra.Command {
	var catUc usecase.CatalogUsecase
	if err := c.Invoke(func(sConf *config.ServiceConfig, uc usecase.CatalogUsecase) {
		catUc = uc
	}); err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove unreachable images from albums",
		RunE: func(cmd *cobra.Command, args []string) error {
			return catUc.PurgeOrphans()
		}}

	return cmd
}
