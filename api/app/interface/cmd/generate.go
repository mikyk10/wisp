package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/usecase"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

func NewGenerateRunCommand(c *dig.Container) *cobra.Command {
	var uc usecase.GenerateUsecase
	if err := c.Invoke(func(u usecase.GenerateUsecase) {
		uc = u
	}); err != nil {
		log.Fatalf("failed to initialize generate: %v", err)
	}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run AI image generation batch",
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, _ := cmd.Flags().GetString("catalog")
			if catalog == "" {
				return errors.New("--catalog is required")
			}
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
			}
			workers, _ := cmd.Flags().GetInt("workers")
			sourceID, _ := cmd.Flags().GetUint64("source-id")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			return uc.Run(context.Background(), usecase.GenerateRunOptions{
				CatalogKey: catalog,
				SourceID:   model.PrimaryKey(sourceID),
				Workers:    workers,
				DryRun:     dryRun,
				Verbose:    verbose,
			})
		},
	}

	cmd.Flags().StringP("catalog", "c", "", "Catalog key (required)")
	cmd.Flags().IntP("workers", "w", 0, "Number of parallel workers")
	cmd.Flags().Uint64("source-id", 0, "Explicit source image ID (0 = random from source_catalog)")
	cmd.Flags().Bool("dry-run", false, "Show what would be done")
	cmd.Flags().BoolP("verbose", "v", false, "Verbose output")

	return cmd
}

func NewGenerateListCommand(c *dig.Container) *cobra.Command {
	var uc usecase.GenerateUsecase
	if err := c.Invoke(func(u usecase.GenerateUsecase) {
		uc = u
	}); err != nil {
		log.Fatalf("failed to initialize generate: %v", err)
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List cached generated images",
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, _ := cmd.Flags().GetString("catalog")
			if catalog == "" {
				return errors.New("--catalog is required")
			}

			entries, err := uc.List(catalog)
			if err != nil {
				return err
			}

			if len(entries) == 0 {
				fmt.Println("No cached entries.")
				return nil
			}

			for _, e := range entries {
				fmt.Printf("ID=%d  type=%s  size=%d bytes  created=%s\n",
					e.ID, e.ContentType, len(e.ImageData), e.CreatedAt.Format("2006-01-02 15:04:05"))
			}
			return nil
		},
	}

	cmd.Flags().StringP("catalog", "c", "", "Catalog key (required)")
	return cmd
}

func NewGenerateCleanCommand(c *dig.Container) *cobra.Command {
	var uc usecase.GenerateUsecase
	if err := c.Invoke(func(u usecase.GenerateUsecase) {
		uc = u
	}); err != nil {
		log.Fatalf("failed to initialize generate: %v", err)
	}

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean up generation data",
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, _ := cmd.Flags().GetString("catalog")
			if catalog == "" {
				return errors.New("--catalog is required")
			}
			failedOnly, _ := cmd.Flags().GetBool("failed")
			return uc.Clean(catalog, failedOnly)
		},
	}

	cmd.Flags().StringP("catalog", "c", "", "Catalog key (required)")
	cmd.Flags().Bool("failed", false, "Only clean failed executions")
	return cmd
}

func NewGenerateFavoriteCommand(c *dig.Container) *cobra.Command {
	var (
		uc     usecase.GenerateUsecase
		svcCfg *config.ServiceConfig
	)
	if err := c.Invoke(func(u usecase.GenerateUsecase, sc *config.ServiceConfig) {
		uc = u
		svcCfg = sc
	}); err != nil {
		log.Fatalf("failed to initialize generate: %v", err)
	}

	cmd := &cobra.Command{
		Use:   "favorite",
		Short: "Export a cached image to a file catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, _ := cmd.Flags().GetString("catalog")
			if catalog == "" {
				return errors.New("--catalog is required")
			}
			cacheID, _ := cmd.Flags().GetUint64("cache-id")
			if cacheID == 0 {
				return errors.New("--cache-id is required")
			}
			dest, _ := cmd.Flags().GetString("dest")
			if dest == "" {
				return errors.New("--dest is required")
			}

			return uc.Favorite(catalog, model.PrimaryKey(cacheID), dest, svcCfg)
		},
	}

	cmd.Flags().StringP("catalog", "c", "", "Source catalog key (required)")
	cmd.Flags().Uint64("cache-id", 0, "Cache entry ID to export (required)")
	cmd.Flags().String("dest", "", "Destination file catalog key (required)")
	return cmd
}
