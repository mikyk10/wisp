package handler

import (
	"net/http"
	"strconv"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"

	"github.com/labstack/echo/v5"
)

type ImageTagsHandler struct {
	taggingRepo repository.TaggingRepository
}

func NewImageTagsHandler(taggingRepo repository.TaggingRepository) *ImageTagsHandler {
	return &ImageTagsHandler{taggingRepo: taggingRepo}
}

// GetTags handles GET /api/images/:id/tags
func (h *ImageTagsHandler) GetTags(c *echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	tags, err := h.taggingRepo.FindTagNamesByImageID(model.PrimaryKey(id))
	if err != nil {
		return c.String(http.StatusInternalServerError, "Internal Error")
	}
	if tags == nil {
		tags = []string{}
	}
	return c.JSON(http.StatusOK, map[string]any{"tags": tags})
}
