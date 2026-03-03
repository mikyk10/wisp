package cmd

import (
	"context"
	"log"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/usecase"

	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

// NewCatalogTaggingResetCommand returns the `catalog tagging reset` subcommand.
func NewCatalogTaggingResetCommand(c *dig.Container) *cobra.Command {
	var resetUc usecase.TaggingResetUsecase
	if err := c.Invoke(func(uc usecase.TaggingResetUsecase) {
		resetUc = uc
	}); err != nil {
		log.Fatalf("failed to initialize tagging reset: %v", err)
	}

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Delete tagging data so images can be re-processed",
		Long: `Delete image_tags, ai_runs, and ai_outputs for a catalog or a single image.

After a reset, a normal 'tagging run' will re-process all affected images.
If the run is interrupted, re-running 'tagging run' resumes from where it left off.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, _ := cmd.Flags().GetString("catalog")
			imageID, _ := cmd.Flags().GetInt64("image-id")

			return resetUc.Run(context.Background(), usecase.TaggingResetOptions{
				CatalogKey: catalog,
				ImageID:    model.PrimaryKey(imageID),
			})
		},
	}

	cmd.Flags().StringP("catalog", "c", "", "Catalog key (required)")
	cmd.Flags().Int64("image-id", 0, "Reset a single image by ID (0 = reset entire catalog)")
	_ = cmd.MarkFlagRequired("catalog")

	return cmd
}
