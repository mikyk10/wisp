package handler

import (
	"net/http"
	"github.com/mikyk10/wisp/app/domain/model"

	"github.com/labstack/echo/v5"
)

type HealthHandler struct{}

func (a HealthHandler) GetIndex(c *echo.Context) error {
	return c.String(http.StatusOK, "OK")
}

func (a HealthHandler) GetVersion(c *echo.Context) error {
	return c.String(http.StatusOK, model.AppVersionString())
}
