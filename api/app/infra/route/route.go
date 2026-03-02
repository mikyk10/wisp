package route

import (
	"errors"
	"log"
	"net/http"
	"wspf/app/interface/handler"
	"wspf/app/interface/handler/response"

	"github.com/labstack/echo/v5"
	"go.uber.org/dig"
)

// Associates each URL to a controller action
func Configure(e *echo.Echo, ctn *dig.Container) *echo.Echo {

	if err := ctn.Invoke(func(h handler.CatalogHandler) { //nolint:contextcheck
		// setup route
		apis := e.Group("")
		{
			// /catalogs
			// Returns the list of catalogs.
			apis.GET("/catalogs", h.ListCatalogs)

			// /catalog/{catalog key}/images
			// Returns the list of indexed images under the catalog.
			apis.GET("/catalog/:catalogKey/images", h.List)

			// /catalog/{catalog key}/image/{ID Number}.{Extention}
			// Returns the specified image in the catalog in the file format indicated by the extension.
			apis.GET("/catalog/:catalogKey/image/:imgid", h.Img)

			// /devices
			// Returns all devices in use.
			apis.GET("/devices", h.List)

			//
			//
			apis.POST("/catalog/selected/_toggle-visibility", h.ToggleVisibility)

			// /pf/{DEVICE_MAC_ADDR}/image/{ID Number}.{Extention}
			// Returns the specified image suitable for the device in the file format indicated by the extension.
			apis.GET("/pf/:displayKey/image/:imgid", h.Img)

			// /pf/{DEVICE_MAC_ADDR}/image/{ID Number}.{Extention}
			// Selects an image from the catalog suitable for the device and returns it in the file format indicated by the extension.
			apis.GET("/pf/:displayKey/image/random.*", h.RandomImg)
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
