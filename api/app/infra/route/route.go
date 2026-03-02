package route

import (
	"errors"
	"log"
	"net/http"
	"github.com/mikyk10/wisp/app/interface/handler"
	"github.com/mikyk10/wisp/app/interface/handler/response"

	"github.com/labstack/echo/v5"
	"go.uber.org/dig"
)

// Associates each URL to a controller action
func Configure(e *echo.Echo, ctn *dig.Container) *echo.Echo {

	if err := ctn.Invoke(func(h handler.CatalogHandler) { //nolint:contextcheck
		// Management API — requires authentication
		api := e.Group("/api")
		{
			// /api/catalogs
			api.GET("/catalogs", h.ListCatalogs)

			// /api/catalog/{catalog key}/images
			api.GET("/catalog/:catalogKey/images", h.List)

			// /api/catalog/{catalog key}/image/{ID Number}.{Extension}
			api.GET("/catalog/:catalogKey/image/:imgid", h.Img)

			// /api/devices
			api.GET("/devices", h.List)

			// /api/catalog/selected/_toggle-visibility
			api.POST("/catalog/selected/_toggle-visibility", h.ToggleVisibility)
		}

		// Device API — no authentication required (called by ESP32 firmware)
		{
			// /pf/{display key}/image/{ID Number}.{Extension}
			e.GET("/pf/:displayKey/image/:imgid", h.Img)

			// /pf/{display key}/image/random.{Extension}
			e.GET("/pf/:displayKey/image/random.*", h.RandomImg)
		}

		pages := e.Group("")
		{
			pages.GET("/health", handler.HealthHandler{}.GetIndex)
			pages.GET("/version", handler.HealthHandler{}.GetVersion)
		}
	}); err != nil {
		log.Fatalf("failed to configure routes: %v", err)
	}

	// uncomment to enable setup route
	e.Static("/", "resources/public")

	// error handler
	e.HTTPErrorHandler = func(c *echo.Context, err error) {
		// Default unhandled errors to 500.
		code := http.StatusInternalServerError
		var he *echo.HTTPError
		if errors.As(err, &he) {
			code = he.Code
		}

		// slog-echo handles logging of unhandled errors, so no additional logging is needed here.

		// Return the error response.
		traceID, _ := c.Get("trace_id").(string)
		response := response.NewErrorResponse(err, traceID) // include trace_id in the JSON response
		if err := c.JSON(code, response); err != nil {
			c.Logger().Error(err.Error())
		}
	}

	return e
}
