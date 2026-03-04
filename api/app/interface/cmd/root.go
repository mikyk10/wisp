package cmd

import (
	"log"
	"log/slog"

	"github.com/goark/gocli/exitcode"
	"github.com/goark/gocli/rwi"
	"github.com/labstack/echo/v5"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

// Global logger.
// slog with context.Context provides structured logging throughout; no need to maintain separate loggers.
var logger *slog.Logger

func newRootCmd(ui *rwi.RWI, args []string) *cobra.Command {
	rootCmd := &cobra.Command{}

	rootCmd.SilenceUsage = true
	rootCmd.SetArgs(args)            //arguments of command-line
	rootCmd.SetIn(ui.Reader())       //Stdin
	rootCmd.SetOut(ui.ErrorWriter()) //Stdout -> Stderr
	rootCmd.SetErr(ui.ErrorWriter()) //Stderr
	return rootCmd
}

func Execute(container *dig.Container, args []string) exitcode.ExitCode {

	var ui *rwi.RWI
	if err := container.Invoke(func(u *rwi.RWI) {
		ui = u
	}); err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}

	rootCmd := newRootCmd(ui, args)
	setupCommands(rootCmd, container)

	if err := rootCmd.Execute(); err != nil {
		return exitcode.Abnormal
	}

	return exitcode.Normal
}

func setupCommands(rootCmd *cobra.Command, container *dig.Container) {
	rootCmd.PersistentFlags().Bool("show-env", false, "show current environmental info")

	// Version
	rootCmd.AddCommand(NewVersionCommand(container))

	systemCmd := &cobra.Command{
		Use:   "system",
		Short: "Manage wisp",
	}
	rootCmd.AddCommand(systemCmd)
	systemCmd.AddCommand(NewSystemPruneCommand(container))

	catalogCmd := &cobra.Command{
		Use:   "catalog",
		Short: "Manage image albums",
	}
	rootCmd.AddCommand(catalogCmd)
	catalogCmd.AddCommand(NewCatalogListCommand(container))
	catalogCmd.AddCommand(NewAlbumScanCommand(container))
	catalogCmd.AddCommand(NewAlbumCleanupCommand(container))
	catalogCmd.AddCommand(NewCatalogListImagesCommand(container))

	taggingCmd := &cobra.Command{Use: "tagging", Short: "AI photo tagging pipeline"}
	catalogCmd.AddCommand(taggingCmd)
	taggingCmd.AddCommand(NewCatalogTaggingRunCommand(container))
	taggingCmd.AddCommand(NewCatalogTaggingResetCommand(container))

	imageCmd := &cobra.Command{
		Use:   "image",
		Short: "Image action",
	}
	rootCmd.AddCommand(imageCmd)
	imageCmd.AddCommand(NewImageConvertCommand(container))

	// Run for Web
	webCmd := &cobra.Command{
		Use:   "web",
		Short: "Web server",
	}
	rootCmd.AddCommand(webCmd)
	webCmd.AddCommand(NewWebRunCommand(container, echo.New()))
}
