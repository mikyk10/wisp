package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"net/http"
	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/repository"
)

// ImageLoader represents an image source.
// It returns one image's data from a managed collection or a single image.
type ImageLoader interface {
	Load() (image.Image, *model.ImgMeta, error)
	GetSourcePath() string
}

// ClearableImageLoader extends ImageLoader for loaders that cache decoded images internally.
// Implement ClearImage() to release the cached image early (before blocking I/O such as DB writes)
// to prevent large image data from being held in goroutine memory unnecessarily.
type ClearableImageLoader interface {
	ImageLoader
	ClearImage()
}

type imageLoader struct {
	img  image.Image
	meta *model.ImgMeta
}

func (i *imageLoader) Load() (image.Image, *model.ImgMeta, error) {
	return i.img, i.meta, nil
}

func (i *imageLoader) GetSourcePath() string {
	return ""
}

// ----
// TODO: if loading cannot be guaranteed, error handling is impossible — this would then be a Pointer, not a Loader
var httpClient = &http.Client{}

type imageURLLoader struct {
	url  string
	img  image.Image
	meta *model.ImgMeta
}

func (i *imageURLLoader) Load() (image.Image, *model.ImgMeta, error) {
	i.meta = &model.ImgMeta{}

	resp, err := httpClient.Get(i.url)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("http status %d from %s", resp.StatusCode, i.url)
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	i.img = img

	//TODO: downstream filters should handle this properly

	return i.img, i.meta, nil
}

func (i *imageURLLoader) GetSourcePath() string {
	return i.url
}

// -----

type imageLocalFilePointer struct {
	*imageLoader
	path string
	epd  epaper.DisplayMetadata
}

func (i *imageLocalFilePointer) Load() (image.Image, *model.ImgMeta, error) {
	if i.img != nil && i.meta != nil {
		return i.img, i.meta, nil
	}

	img, meta, err := load(i.path)
	if err != nil {
		return nil, nil, err
	}

	i.img = img
	i.meta = meta

	return i.img, i.meta, nil
}

func (i *imageLocalFilePointer) GetSourcePath() string {
	return i.path
}

// ClearImage releases the cached decoded image to allow GC.
// Call this after thumbnail generation is complete and before any blocking operations (e.g. DB writes)
// to avoid holding large images in memory while waiting for I/O.
func (i *imageLocalFilePointer) ClearImage() {
	i.img = nil
}

// -----

// imageDBLoader loads an image from the database image_data column.
// Used for background HTTP catalog images.
type imageDBLoader struct {
	id   model.PrimaryKey
	url  string
	repo repository.ImageRepository
	img  image.Image
	meta *model.ImgMeta
}

func (i *imageDBLoader) Load() (image.Image, *model.ImgMeta, error) {
	data, err := i.repo.FindImageData(i.id)
	if err != nil {
		return nil, nil, err
	}
	if len(data) == 0 {
		return nil, nil, errors.New("image_data is empty")
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}

	i.img = img
	i.meta = &model.ImgMeta{}
	return i.img, i.meta, nil
}

func (i *imageDBLoader) GetSourcePath() string {
	return i.url
}
