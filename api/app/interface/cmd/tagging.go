package cmd

import (
	"context"
	"log"

	"github.com/mikyk10/wisp/app/usecase"

	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

// NewCatalogTaggingRunCommand returns the `catalog tagging run` subcommand.
func NewCatalogTaggingRunCommand(c *dig.Container) *cobra.Command {
	var pipelineUc usecase.TaggingPipelineUsecase
	if err := c.Invoke(func(uc usecase.TaggingPipelineUsecase) {
		pipelineUc = uc
	}); err != nil {
		log.Fatalf("failed to initialize tagging pipeline: %v", err)
	}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the AI photo tagging pipeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, _ := cmd.Flags().GetString("catalog")
			workers, _ := cmd.Flags().GetInt("workers")
			limit, _ := cmd.Flags().GetInt("limit")
			rebuild, _ := cmd.Flags().GetBool("rebuild")
			stage, _ := cmd.Flags().GetInt("stage")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			descriptorPromptPath, _ := cmd.Flags().GetString("descriptor-prompt-path")
			taggerPromptPath, _ := cmd.Flags().GetString("tagger-prompt-path")

			return pipelineUc.Run(context.Background(), usecase.TaggingRunOptions{
				CatalogKey:           catalog,
				Workers:              workers,
				Limit:                limit,
				Rebuild:              rebuild,
				Stage:                stage,
				DryRun:               dryRun,
				DescriptorPromptPath: descriptorPromptPath,
				TaggerPromptPath:     taggerPromptPath,
			})
		},
	}

	cmd.Flags().StringP("catalog", "c", "", "Catalog key (required)")
	cmd.Flags().IntP("workers", "w", 0, "Number of parallel workers (0 = config default)")
	cmd.Flags().Int("limit", 0, "Maximum number of images to process (0 = no limit)")
	cmd.Flags().Bool("rebuild", false, "Re-tag all images, including already-tagged ones")
	cmd.Flags().Int("stage", 0, "Start from stage N (0 = all; 2 = tagging only, requires --rebuild)")
	cmd.Flags().Bool("dry-run", false, "Show what would be done without writing to the DB")
	cmd.Flags().String("descriptor-prompt-path", "", "Path to a custom descriptor prompt .md file (default: built-in)")
	cmd.Flags().String("tagger-prompt-path", "", "Path to a custom tagger prompt .md file (default: built-in)")

	_ = cmd.MarkFlagRequired("catalog")

	return cmd
}
