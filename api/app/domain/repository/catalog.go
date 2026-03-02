package repository

import (
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
)

type ImageFileRepository interface {
	GetImagePointer(config.ImageFileProviderConfig) *model.Image
}
