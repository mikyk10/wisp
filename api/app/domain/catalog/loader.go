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

// colorbarSource is what the generated test pattern names as its source. There is
// no file behind it, but a loader still has to say where its picture came from.
const colorbarSource = "generated"

// Provenance identifies the picture a loader will produce: which delivery path
// built it, which catalog row it was drawn from, and — when it is an error card —
// why the card was drawn instead of a photo.
//
// Each loader reports its own kind, and no caller may infer it. A provider that
// gives up returns an error-card loader with a nil error (see
// imageIndexedFileProvider.Resolve, which does this for three separate
// failures), so guessing the kind from which provider was asked, or from the
// fact that Resolve succeeded, records exactly those failures as photos.
type Provenance struct {
	// Kind is the delivery path that produced the picture.
	Kind model.DeliveryKind

	// Reason says why an error card was produced. It is empty for every other
	// kind.
	Reason model.DeliveryReason

	// ImageID is the images row behind the picture, or 0 when there is none:
	// a live HTTP fetch, a color bar, an error card, or a file not yet indexed.
	ImageID model.PrimaryKey

	// Source is the file path or URL the picture came from, empty when there is
	// none.
	Source string

	// CatalogKey is the catalog that was consulted, empty when none was.
	CatalogKey string
}

// ImageLoader represents an image source.
// It returns one image's data from a managed collection or a single image.
type ImageLoader interface {
	Load() (image.Image, *model.ImgMeta, error)

	// Provenance is settled when the loader is built, so it stays readable after
	// Load has run or has failed.
	Provenance() Provenance
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
	prov Provenance
}

// newErrorCardLoader wraps an error card that has already been rendered. The
// reason comes from whoever gave up: the failure is over by the time the card
// exists, and the pixels say nothing about what went wrong.
func newErrorCardLoader(img image.Image, meta *model.ImgMeta, reason model.DeliveryReason, catalogKey string) *imageLoader {
	return &imageLoader{
		img:  img,
		meta: meta,
		prov: Provenance{
			Kind:       model.DeliveryKindError,
			Reason:     reason,
			CatalogKey: catalogKey,
		},
	}
}

func (i *imageLoader) Load() (image.Image, *model.ImgMeta, error) {
	return i.img, i.meta, nil
}

func (i *imageLoader) Provenance() Provenance {
	return i.prov
}

// ----
// TODO: if loading cannot be guaranteed, error handling is impossible — this would then be a Pointer, not a Loader
var httpClient = &http.Client{}

type imageURLLoader struct {
	img  image.Image
	meta *model.ImgMeta
	prov Provenance
}

// newRealtimeHTTPLoader builds a loader that fetches from url on demand. Nothing
// is stored for it, so there is no images row to point at.
func newRealtimeHTTPLoader(url, catalogKey string) *imageURLLoader {
	return &imageURLLoader{
		prov: Provenance{
			Kind:       model.DeliveryKindHTTP,
			Source:     url,
			CatalogKey: catalogKey,
		},
	}
}

func (i *imageURLLoader) Load() (image.Image, *model.ImgMeta, error) {
	i.meta = &model.ImgMeta{}

	resp, err := httpClient.Get(i.prov.Source)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("http status %d from %s", resp.StatusCode, i.prov.Source)
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	i.img = img

	//TODO: downstream filters should handle this properly

	return i.img, i.meta, nil
}

func (i *imageURLLoader) Provenance() Provenance {
	return i.prov
}

// -----

// imageLocalFilePointer reads a picture from a path on disk, or hands back one
// that was decoded already. Build it through the constructors below: they are the
// only place a kind is chosen, so a loader cannot reach a caller without one.
type imageLocalFilePointer struct {
	*imageLoader
	epd epaper.DisplayMetadata
}

// newIndexedPhotoLoader builds a loader for a photo chosen from the catalog
// index, carrying the row it was chosen from.
func newIndexedPhotoLoader(rec *model.Image, catalogKey string, epd epaper.DisplayMetadata) *imageLocalFilePointer {
	return &imageLocalFilePointer{
		imageLoader: &imageLoader{
			prov: Provenance{
				Kind:       model.DeliveryKindPhoto,
				ImageID:    rec.ID,
				Source:     rec.Src,
				CatalogKey: catalogKey,
			},
		},
		epd: epd,
	}
}

// newLocalPhotoLoader builds a loader for a file named directly — a CLI argument,
// or a path turned up by a scan — which has no catalog row behind it yet.
// img and meta may be nil, in which case the file is read on the first Load.
func newLocalPhotoLoader(path string, img image.Image, meta *model.ImgMeta, epd epaper.DisplayMetadata) *imageLocalFilePointer {
	return &imageLocalFilePointer{
		imageLoader: &imageLoader{
			img:  img,
			meta: meta,
			prov: Provenance{
				Kind:   model.DeliveryKindPhoto,
				Source: path,
			},
		},
		epd: epd,
	}
}

// newColorbarLoader builds a loader for the generated test pattern. It reuses the
// local-file loader but has no file, so the kind has to be stated here: the path
// it carries is a placeholder and would read as a photo to anyone downstream.
func newColorbarLoader(img image.Image, meta *model.ImgMeta, epd epaper.DisplayMetadata) *imageLocalFilePointer {
	return &imageLocalFilePointer{
		imageLoader: &imageLoader{
			img:  img,
			meta: meta,
			prov: Provenance{
				Kind:   model.DeliveryKindColorbar,
				Source: colorbarSource,
			},
		},
		epd: epd,
	}
}

func (i *imageLocalFilePointer) Load() (image.Image, *model.ImgMeta, error) {
	if i.img != nil && i.meta != nil {
		return i.img, i.meta, nil
	}

	img, meta, err := load(i.prov.Source)
	if err != nil {
		return nil, nil, err
	}

	i.img = img
	i.meta = meta

	return i.img, i.meta, nil
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
	repo repository.ImageRepository
	img  image.Image
	meta *model.ImgMeta
	prov Provenance
}

// newCachedHTTPLoader builds a loader for an HTTP picture already fetched into
// the catalog, read back from the image_data column of the row it was stored in.
func newCachedHTTPLoader(rec *model.Image, catalogKey string, repo repository.ImageRepository) *imageDBLoader {
	return &imageDBLoader{
		repo: repo,
		prov: Provenance{
			Kind:       model.DeliveryKindHTTP,
			ImageID:    rec.ID,
			Source:     rec.Src,
			CatalogKey: catalogKey,
		},
	}
}

func (i *imageDBLoader) Load() (image.Image, *model.ImgMeta, error) {
	data, err := i.repo.FindImageData(i.prov.ImageID)
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

func (i *imageDBLoader) Provenance() Provenance {
	return i.prov
}
