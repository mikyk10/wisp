package cmd

import (
	"context"
	"log"
	"log/slog"
	"maps"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/repository"
	"github.com/mikyk10/wisp/app/infra"
	"github.com/mikyk10/wisp/app/infra/route"

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
			var serviceConfig *config.ServiceConfig
			if err := c.Invoke(func(gConf *config.GlobalConfig, sConf *config.ServiceConfig, rwi *rwi.RWI) {
				globalConfig = gConf
				serviceConfig = sConf

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

			reconcileDeliveryHistory(ctx, c, globalConfig, serviceConfig)

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

// reconcileDeliveryHistory brings the stored delivery history back inside the
// bounds this configuration gives it: rows past the end of a ring that has been
// made smaller, and rows belonging to displays no longer named in service.yaml.
//
// Start-up is the whole of the coverage this needs, and it is not cosmetic.
// Shrinking the size removes nothing by itself — the higher slots simply stop
// being written to — so without this the table would sit above its configured
// bound for good, which is the one thing a fixed-size ring exists to prevent.
// The configuration is read once, in main, and there is no reload path, so
// neither the size nor the set of displays can change while the process runs.
//
// It runs even when the feature is disabled. Disabling stops new deliveries
// being written down; it does not promise to keep the rows written while it was
// on, and with the recording path switched off nothing else will ever trim
// them. The deletes are bounded by the same configured size and display keys
// either way, so this removes nothing the enabled path would have kept.
//
// Nothing here may stop the server. The likeliest failure by far is the table
// not existing at all: WISP_AUTO_MIGRATE is off by default, so the first boot
// after this upgrade meets a missing table on every installation that predates
// the feature. That is the expected case, not an exceptional one, and it is no
// reason to refuse to serve pictures.
func reconcileDeliveryHistory(ctx context.Context, c *dig.Container, gConf *config.GlobalConfig, sConf *config.ServiceConfig) {
	if gConf == nil || sConf == nil {
		return
	}

	// The map keys, deliberately, and not anything derived from the display.
	// A display is keyed by its mac_address, which is the same string the
	// device puts in the :displayKey path parameter and the same one
	// RecordDelivery files its row under. A key that did not match would make
	// every configured display look retired and delete the lot on every boot.
	// Sorted only so the statement is stable between runs.
	displayKeys := slices.Sorted(maps.Keys(sConf.Displays))

	if err := c.Invoke(func(dhr repository.DeliveryHistoryRepository) {
		if err := dhr.Reconcile(displayKeys, gConf.DeliveryHistory.Size); err != nil {
			slog.WarnContext(ctx, "delivery history: reconcile failed",
				"displays", len(displayKeys), "size", gConf.DeliveryHistory.Size, "err", err)
		}
	}); err != nil {
		slog.WarnContext(ctx, "delivery history: reconcile skipped", "err", err)
	}
}
