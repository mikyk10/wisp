package cmd

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"wspf/app/domain/model"
	"wspf/app/domain/model/config"
	"wspf/app/infra"
	"wspf/app/infra/route"

	slogcontext "github.com/PumpkinSeed/slog-context"
	"github.com/goark/gocli/rwi"
	"github.com/gofrs/uuid"
	"github.com/labstack/echo/v5"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

// setup logger
func setupSlogForAPIServer(rwi *rwi.RWI, gConf *config.GlobalConfig) {

	handler := slogcontext.NewHandler(
		slog.NewJSONHandler(rwi.ErrorWriter(), &slog.HandlerOptions{
			// Include the source location of each log statement.
			AddSource: true,
			Level:     gConf.LogLevel,
		}),
	)

	logger = slog.New(handler)

	// Attach environment name and version to all log output.
	logger = logger.With("env", gConf.Env)

	// Set the global default logger so it can be used wherever context.Context is available.
	slog.SetDefault(logger)
}

func NewWebRunCommand(c *dig.Container, e *echo.Echo) *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run the Web application",
		Run: func(cmd *cobra.Command, args []string) {

			var globalConfig *config.GlobalConfig
			if err := c.Invoke(func(gConf *config.GlobalConfig, sConf *config.ServiceConfig, rwi *rwi.RWI) {
				globalConfig = gConf

				setupSlogForAPIServer(rwi, gConf)
			}); err != nil {
				log.Fatalf("failed to initialize web server: %v", err)
			}

			// Log server start and stop.
			serverID, _ := uuid.NewV4()
			ctx := slogcontext.WithValue(context.Background(), "server_id", serverID.String())
			slog.InfoContext(ctx, "server starting...", "ver", model.AppVersionString())

			logger = logger.With("ver", model.AppShortVersionString())
			slog.SetDefault(logger)

			e := echo.New()
			e = infra.Middlewares(logger, route.Configure(e, c))

			// Graceful shutdown via context cancellation (echo v5 StartConfig)
			startCtx, cancel := signal.NotifyContext(ctx, os.Interrupt)
			defer cancel()

			sc := echo.StartConfig{
				Address:    ":" + strconv.Itoa(globalConfig.Port),
				HideBanner: true,
				HidePort:   true,
			}

			slog.InfoContext(ctx, "server started", "port", globalConfig.Port)

			if err := sc.Start(startCtx, e); err != nil {
				slog.ErrorContext(ctx, err.Error())
			}

			slog.InfoContext(ctx, "server stopped")
		}}
}
