package infra

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	slogcontext "github.com/PumpkinSeed/slog-context"
	"github.com/gofrs/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func Middlewares(logger *slog.Logger, e *echo.Echo) *echo.Echo {

	// Middlewares
	// All panics should carefully be handled in the first place.
	e.Use(middleware.Recover())

	// Access Logging (echo v5 built-in RequestLogger with slog)
	accessLogger := logger.With(slog.String("type", "access"))
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		Skipper: func(c *echo.Context) bool {
			return c.Path() == "/health"
		},
		LogMethod:    true,
		LogURI:       true,
		LogStatus:    true,
		LogLatency:   true,
		LogRequestID: true,
		LogHeaders:   []string{"Accept", "Content-Type"},
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			level := slog.LevelInfo
			if v.Error != nil {
				level = slog.LevelError
			}
			attrs := []slog.Attr{
				slog.String("method", v.Method),
				slog.String("uri", v.URI),
				slog.Int("status", v.Status),
				slog.Duration("latency", v.Latency),
				slog.String("request_id", v.RequestID),
			}
			if v.Error != nil {
				attrs = append(attrs, slog.String("error", v.Error.Error()))
			}
			accessLogger.LogAttrs(context.Background(), level, "REQUEST", attrs...)
			return nil
		},
	}))

	e.Use(middleware.RequestIDWithConfig(middleware.RequestIDConfig{
		Generator: func() string {
			uuid, _ := uuid.NewV4()
			return uuid.String()
		},
		RequestIDHandler: func(c *echo.Context, rid string) {
			// Propagate trace_id into the context so logs are correlated with the request scope.
			c.Set("trace_id", rid)
			ctx := slogcontext.WithValue(c.Request().Context(), "trace_id", rid)

			newReq := c.Request().WithContext(ctx)
			c.SetRequest(newReq)
		},
	}))

	e.Use(middleware.ContextTimeoutWithConfig(middleware.ContextTimeoutConfig{
		Timeout: 30 * time.Second,
	}))

	e.Use(middleware.Decompress()) // handle gzipped requests
	e.Use(middleware.BodyLimit(2 * 1024 * 1024))

	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Level: 5,
	}))

	// CORS: defaults to "*" (allow all origins).
	// Set ALLOWED_ORIGINS to a comma-separated list of allowed origins to restrict
	// cross-origin access in production (e.g. ALLOWED_ORIGINS=https://your-frontend.example.com).
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "*"
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: strings.Split(allowedOrigins, ","),
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	return e
}
