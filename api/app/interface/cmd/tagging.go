package cmd

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"os"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/usecase"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

func NewTaggingRunCommand(c *dig.Container) *cobra.Command {
	var uc usecase.TaggingUsecase
	if err := c.Invoke(func(u usecase.TaggingUsecase) {
		uc = u
	}); err != nil {
		log.Fatalf("failed to initialize tagging: %v", err)
	}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run AI tagging pipeline on catalog images",
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
			limit, _ := cmd.Flags().GetInt("limit")
			rebuild, _ := cmd.Flags().GetBool("rebuild")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			return uc.Run(context.Background(), usecase.TaggingRunOptions{
				CatalogKey: catalog,
				Workers:    workers,
				Limit:      limit,
				Rebuild:    rebuild,
				DryRun:     dryRun,
				Verbose:    verbose,
			})
		},
	}

	cmd.Flags().StringP("catalog", "c", "", "Catalog key (required)")
	cmd.Flags().IntP("workers", "w", 0, "Number of parallel workers (0 = config default)")
	cmd.Flags().Int("limit", 0, "Maximum images to process (0 = no limit)")
	cmd.Flags().Bool("rebuild", false, "Re-tag all images")
	cmd.Flags().Bool("dry-run", false, "Show what would be done without executing")
	cmd.Flags().BoolP("verbose", "v", false, "Per-image debug output")

	return cmd
}

func NewTaggingResetCommand(c *dig.Container) *cobra.Command {
	var uc usecase.TaggingUsecase
	if err := c.Invoke(func(u usecase.TaggingUsecase) {
		uc = u
	}); err != nil {
		log.Fatalf("failed to initialize tagging: %v", err)
	}

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset tagging data for a catalog or single image",
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, _ := cmd.Flags().GetString("catalog")
			if catalog == "" {
				return errors.New("--catalog is required")
			}
			imageID, _ := cmd.Flags().GetUint64("image-id")
			return uc.Reset(catalog, model.PrimaryKey(imageID))
		},
	}

	cmd.Flags().StringP("catalog", "c", "", "Catalog key (required)")
	cmd.Flags().Uint64("image-id", 0, "Reset single image (0 = entire catalog)")

	return cmd
}
