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

	if err := ctn.Invoke(func(h handler.CatalogHandler, d handler.DeviceHandler) { //nolint:contextcheck
		// Management API — served to the operator's browser. Nothing here is
		// authenticated: Middlewares() installs no check on the caller, so
		// anything that can reach the port can call all of it.
		api := e.Group("/api")
		{
			// /api/catalogs
			api.GET("/catalogs", h.ListCatalogs)

			// /api/catalog/{catalog key}/images
			api.GET("/catalog/:catalogKey/images", h.List)

			// Tags available in one catalogue. Its own route rather than a
			// field on the catalogue list, because it is read when the picker
			// opens and not when the page loads.
			api.GET("/catalog/:catalogKey/tags", h.ListTags)

			// /api/catalog/{catalog key}/image/{ID Number}.{Extension}
			api.GET("/catalog/:catalogKey/image/:imgid", h.ImgManagement)

			// /api/devices
			api.GET("/devices", d.ListDevices)

			// /api/device/{display key}/deliveries
			api.GET("/device/:displayKey/deliveries", d.ListDeliveries)

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
		} else if err.Error() == "Not Found" {
			// Handle "Not Found" from static file handler
			code = http.StatusNotFound
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
