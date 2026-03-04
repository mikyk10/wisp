package catalog

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
	"github.com/mikyk10/wisp/app/domain/display/epaper"
	"github.com/mikyk10/wisp/app/domain/finder/fs"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/repository"

	"gorm.io/gorm"
)

func NewImageIndexedFileProvider(now time.Time, epd epaper.DisplayMetadata, repo repository.ImageRepository, catalogKey string, fileconf config.ImageFileProviderConfig) ImageLocator {
	return &imageIndexedFileProvider{
		now:                now,
		epd:                epd,
		repo:               repo,
		catalogKey:         catalogKey,
		fileProviderConfig: fileconf,
	}
}

// imageIndexedFileProvider resolves an image path based on the catalog index.
// Image data is retrieved from local files, as with imageLocalFileProvider, but the resolution method differs.
type imageIndexedFileProvider struct {
	now                time.Time
	epd                epaper.DisplayMetadata
	repo               repository.ImageRepository
	catalogKey         string
	fileProviderConfig config.ImageFileProviderConfig
}

func (i *imageIndexedFileProvider) Resolve() (ImageLoader, error) {
	nfProviderFunc := func(msg string) ImageLocator {
		return &imageErrorMessageProvider{i.epd, &config.ImageErrorMessageProviderConfig{
			Message: msg,
		}, nil}
	}

	selectedImage, err := i.repo.FindByRandom(i.catalogKey, i.epd.InstalledOrientation())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nfProviderFunc("No images indexed").Resolve()
		}
		slog.Error("FindByRandom failed", "catalog", i.catalogKey, "err", err)
		return nfProviderFunc("DB error").Resolve()
	}

	//TODO: repository access
	//TODO: take one image path from it
	//TODO: open file and load the image
	//TODO: if something goes wrong, return the result of imageNotFoundProvider.Resolve()

	if _, err := os.Stat(selectedImage.Src); err != nil {
		slog.Error("file not found", "path", selectedImage.Src, "err", err)
		return nfProviderFunc("Image unavailable").Resolve()
	}

	return &imageLocalFilePointer{
		&imageLoader{},
		selectedImage.Src,
		i.epd,
	}, nil
}

func (i *imageIndexedFileProvider) EnumerateImages(ctx context.Context, found chan<- ImageLoader, excluded chan<- ImageLoader) {
	defer close(found)
	defer close(excluded)
}

// NewFileImageLocator returns an ImageLocator for a single local image file.
// Unlike NewImageLocalFileProviderFactory, this returns an error (not an error-message image)
// when the file cannot be loaded, making it suitable for CLI use.
func NewFileImageLocator(path string) ImageLocator {
	return &singleFileLocator{path: path}
}

type singleFileLocator struct{ path string }

func (s *singleFileLocator) Resolve() (ImageLoader, error) {
	img, meta, err := load(s.path)
	if err != nil {
		return nil, err
	}
	return &imageLocalFilePointer{&imageLoader{img: img, meta: meta}, s.path, nil}, nil
}

func NewImageLocalFileProviderFactory(now time.Time, conf config.ImageFileProviderConfig) func(path string) BatchImageSource {
	return func(path string) BatchImageSource {
		return &imageLocalFileProvider{
			now:            now,
			targetPath:     path,
			providerConfig: conf,
		}
	}
}

// imageLocalFileProvider resolves an image from a path passed via CLI arguments or similar.
type imageLocalFileProvider struct {
	now            time.Time
	epd            epaper.DisplayMetadata
	targetPath     string
	providerConfig config.ImageFileProviderConfig
}

func (i *imageLocalFileProvider) Resolve() (ImageLoader, error) {

	nfProviderFunc := func(msg string) ImageLocator {
		return &imageErrorMessageProvider{i.epd, &config.ImageErrorMessageProviderConfig{
			Message: msg,
		}, nil}
	}

	img, meta, err := load(i.targetPath)
	if err != nil {
		return nfProviderFunc(err.Error()).Resolve()
	}

	return &imageLocalFilePointer{
		&imageLoader{
			img:  img,
			meta: meta,
		},
		i.targetPath,
		i.epd,
	}, nil
}

//nolint:cyclop // TODO: refactor
func (i *imageLocalFileProvider) EnumerateImages(ctx context.Context, found chan<- ImageLoader, excluded chan<- ImageLoader) {
	defer close(found)
	defer close(excluded)

	fsChan := make(chan string, cap(found))
	finder := fs.NewFsImageFilePathFinder(i.providerConfig.SrcPath)
	go finder.Find(ctx, fsChan)

	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, cap(found))

	for path := range fsChan {

		select {
		case <-ctx.Done():
			return
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(path string) {
			defer func() {
				wg.Done()
				<-sem
			}()

			provider, included := func(path string) (ImageLoader, bool) {

				imgPtr := &imageLocalFilePointer{
					&imageLoader{},
					path,
					i.epd,
				}
				meta, err := loadMeta(path)
				if err != nil {
					slog.ErrorContext(ctx, "failed to load image", "path", path, "error", err)
					return imgPtr, false
				}

				// Exclude items without a timestamp when an ExifDateTime criterion is specified.
				if len(i.providerConfig.Criteria.Include.ExifTimeRange) > 0 || len(i.providerConfig.Criteria.Exclude.ExifTimeRange) > 0 {
					if meta.ExifDateTime.IsZero() {
						slog.DebugContext(ctx, "ExifDateTime is zero", "path", path)
						return imgPtr, false
					}
				}

				if len(i.providerConfig.Criteria.Include.Path) > 0 {
					cnt := 0
					for _, excl := range i.providerConfig.Criteria.Include.Path {
						if strings.Contains(path, excl) {
							cnt++
						}
					}
					if cnt == 0 {
						return imgPtr, false
					}
				}

				for _, excl := range i.providerConfig.Criteria.Exclude.Path {
					if strings.Contains(path, excl) {
						return imgPtr, false
					}
				}

				if len(i.providerConfig.Criteria.Include.ExifTimeRange) > 0 {
					cnt := 0
					for _, incl := range i.providerConfig.Criteria.Include.ExifTimeRange {
						//if meta.ExifDateTime.IsZero() {
						//	continue
						//}

						if incl.Last > 0 {
							if timeLastN(meta.ExifDateTime, i.now, incl.Last) {
								cnt++
							}
						}

						if timeIsBetween(meta.ExifDateTime, incl.From, incl.To) {
							cnt++
						}
					}

					if cnt == 0 {
						return imgPtr, false
					}
				}

				for _, excl := range i.providerConfig.Criteria.Exclude.ExifTimeRange {
					//if meta.ExifDateTime.IsZero() {
					//	continue
					//}

					if excl.Last > 0 {
						if timeLastN(meta.ExifDateTime, i.now, excl.Last) {
							return imgPtr, false
						}
					}

					if timeIsBetween(meta.ExifDateTime, excl.From, excl.To) {
						return imgPtr, false
					}
				}

				return imgPtr, true
			}(path)

			if included {
				found <- provider
			} else {
				excluded <- provider
			}
		}(path)
	}

	wg.Wait()
}

func timeIsBetween(t, tmin, tmax time.Time) bool {
	if tmin.After(tmax) {
		tmin, tmax = tmax, tmin
	}
	return (t.Equal(tmin) || t.After(tmin)) && (t.Equal(tmax) || t.Before(tmax))
}

func timeLastN(subject, needle time.Time, duration time.Duration) bool {
	tmin := needle.Add(-duration)
	tmax := needle
	return timeIsBetween(subject, tmin, tmax)
}
