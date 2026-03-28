package cmd

import (
	"log"
	"log/slog"
	"os"

	"github.com/mikyk10/wisp/app/usecase"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

func NewCatalogFetchCommand(c *dig.Container) *cobra.Command {
	var catUc usecase.CatalogUsecase
	if err := c.Invoke(func(uc usecase.CatalogUsecase) {
		catUc = uc
	}); err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch images from background HTTP catalogs",
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				})))
			}
			catalogKey, _ := cmd.Flags().GetString("catalog")
			workers, _ := cmd.Flags().GetInt("workers")
			maxRetries, _ := cmd.Flags().GetInt("max-retries")
			return catUc.Fetch(catalogKey, workers, maxRetries, verbose)
		},
	}
	cmd.Flags().StringP("catalog", "c", "", "Specific catalog to fetch (empty = all background HTTP catalogs)")
	cmd.Flags().IntP("workers", "w", 0, "Number of parallel fetch goroutines (0 = auto)")
	cmd.Flags().Int("max-retries", 3, "Max retries per fetch (exponential backoff)")
	cmd.Flags().BoolP("verbose", "v", false, "Enable debug logging")

	return cmd
}
