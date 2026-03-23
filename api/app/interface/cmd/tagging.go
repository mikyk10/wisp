package cmd

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/mikyk10/wisp/app/domain/model"
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
			stage, _ := cmd.Flags().GetString("stage")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			verbose, _ := cmd.Flags().GetBool("verbose")
			descriptorPromptPath, _ := cmd.Flags().GetString("descriptor-prompt-path")
			taggerPromptPath, _ := cmd.Flags().GetString("tagger-prompt-path")

			if verbose {
				slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				})))
			}

			return pipelineUc.Run(context.Background(), usecase.TaggingRunOptions{
				CatalogKey:           catalog,
				Workers:              workers,
				Limit:                limit,
				Rebuild:              rebuild,
				Stage:                model.AIRunStage(stage),
				DryRun:               dryRun,
				Verbose:              verbose,
				DescriptorPromptPath: descriptorPromptPath,
				TaggerPromptPath:     taggerPromptPath,
			})
		},
	}

	cmd.Flags().StringP("catalog", "c", "", "Catalog key (required)")
	cmd.Flags().IntP("workers", "w", 0, "Number of parallel workers (0 = config default)")
	cmd.Flags().Int("limit", 0, "Maximum number of images to process (0 = no limit)")
	cmd.Flags().Bool("rebuild", false, "Re-tag all images, including already-tagged ones")
	cmd.Flags().String("stage", "", `Start from stage (""  = all; "tagging" = tagging only, requires --rebuild)`)
	cmd.Flags().Bool("dry-run", false, "Show what would be done without writing to the DB")
	cmd.Flags().BoolP("verbose", "v", false, "Print per-image stage decisions and LLM outputs")
	cmd.Flags().String("descriptor-prompt-path", "", "Path to a custom descriptor prompt .md file (default: built-in)")
	cmd.Flags().String("tagger-prompt-path", "", "Path to a custom tagger prompt .md file (default: built-in)")

	_ = cmd.MarkFlagRequired("catalog")

	return cmd
}
