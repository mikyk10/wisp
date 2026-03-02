package catalog

import (
	"errors"
	"image"
	"net/http"
	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/model"
)

// ImageLoader represents an image source.
// It returns one image's data from a managed collection or a single image.
type ImageLoader interface {
	Load() (image.Image, *model.ImgMeta, error)
	GetSourcePath() string
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
		return nil, nil, errors.New("failed to load image")
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
