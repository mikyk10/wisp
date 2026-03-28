package cmd

import (
	"log"
	"log/slog"
	"os"

	"github.com/mikyk10/wisp/app/usecase"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

func NewTaggingRunCommand(c *dig.Container) *cobra.Command {
	var tagUc usecase.TaggingUsecase
	if err := c.Invoke(func(uc usecase.TaggingUsecase) {
		tagUc = uc
	}); err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Tag untagged images via AI service",
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				})))
			}
			catalogKey, _ := cmd.Flags().GetString("catalog")
			workers, _ := cmd.Flags().GetInt("workers")
			limit, _ := cmd.Flags().GetInt("limit")
			return tagUc.Run(catalogKey, workers, limit)
		},
	}
	cmd.Flags().StringP("catalog", "c", "", "Specific catalog (empty = all file catalogs)")
	cmd.Flags().IntP("workers", "w", 0, "Number of parallel workers (0 = auto)")
	cmd.Flags().IntP("limit", "l", 0, "Max images to tag (0 = unlimited)")
	cmd.Flags().BoolP("verbose", "v", false, "Enable debug logging")

	return cmd
}
