package repository

import (
	"wspf/app/domain/model"
	"wspf/app/domain/model/config"
)

type ImageFileRepository interface {
	GetImagePointer(config.ImageFileProviderConfig) *model.Image
}
