package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/mikyk10/wisp/app/domain/model/config"
	"github.com/mikyk10/wisp/app/domain/repository"
	"github.com/sunshineplan/imgconv"
)

type TaggingUsecase interface {
	Run(catalogKey string, workers int, limit int) error
}

type taggingUsecase struct {
	globalCfg *config.GlobalConfig
	svcCfg    *config.ServiceConfig
	imgRepo   repository.ImageRepository
	tagRepo   repository.TagRepository
}

func NewTaggingUsecase(globalCfg *config.GlobalConfig, svcCfg *config.ServiceConfig, imgRepo repository.ImageRepository, tagRepo repository.TagRepository) TaggingUsecase {
	return &taggingUsecase{globalCfg: globalCfg, svcCfg: svcCfg, imgRepo: imgRepo, tagRepo: tagRepo}
}

func (u *taggingUsecase) Run(catalogKey string, workers int, limit int) error {
	endpoint := u.globalCfg.Tagging.Endpoint
	if endpoint == "" {
		return fmt.Errorf("tagging.endpoint is not configured")
	}

	timeoutSec := u.globalCfg.Tagging.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 180
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sch := make(chan os.Signal, 1)
	signal.Notify(sch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sch)
	go func() {
		select {
		case <-sch:
			slog.Info("tagging: received signal, cancelling")
			cancel()
		case <-ctx.Done():
		}
	}()

	// Collect file catalogs to process.
	var catalogs []string
	if catalogKey != "" {
		catalogs = []string{catalogKey}
	} else {
		for key, cat := range u.svcCfg.Catalog {
			if _, ok := cat.Config.(config.ImageFileProviderConfig); ok {
				catalogs = append(catalogs, key)
			}
		}
	}

	if len(catalogs) == 0 {
		slog.Info("tagging: no catalogs to process")
		return nil
	}

	concurrency := workers
	if concurrency <= 0 {
		concurrency = 1
	}

	for _, cat := range catalogs {
		if ctx.Err() != nil {
			break
		}
		if err := u.tagCatalog(ctx, cat, concurrency, limit, endpoint, timeoutSec); err != nil {
			slog.Error("tagging: catalog failed", "catalog", cat, "err", err)
		}
	}

	return nil
}

func (u *taggingUsecase) tagCatalog(ctx context.Context, catalogKey string, concurrency int, limit int, endpoint string, timeoutSec int) error {
	ids, err := u.tagRepo.FindImagesWithoutTags(catalogKey, limit)
	if err != nil {
		return fmt.Errorf("find untagged images: %w", err)
	}

	if len(ids) == 0 {
		slog.Info("tagging: no untagged images", "catalog", catalogKey)
		return nil
	}

	slog.Info("tagging: started", "catalog", catalogKey, "images", len(ids))

	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, concurrency)
	var processed int
	var mu sync.Mutex

	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(imageID model.PrimaryKey) {
			defer func() { wg.Done(); <-sem }()
			if err := u.tagOne(ctx, imageID, endpoint, timeoutSec); err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Error("tagging: image failed", "image_id", imageID, "err", err)
				return
			}
			if ctx.Err() != nil {
				return
			}
			mu.Lock()
			processed++
			slog.Info("tagging: progress", "catalog", catalogKey, "processed", processed, "total", len(ids))
			mu.Unlock()
		}(id)
	}

	wg.Wait()
	slog.Info("tagging: completed", "catalog", catalogKey, "processed", processed, "total", len(ids))
	return ctx.Err()
}

func (u *taggingUsecase) tagOne(ctx context.Context, imageID model.PrimaryKey, endpoint string, timeoutSec int) error {
	imgBytes, err := u.loadTaggingImage(imageID)
	if err != nil {
		return err
	}

	tags, err := u.callTaggingService(ctx, endpoint, imgBytes, timeoutSec)
	if err != nil {
		return err
	}

	if len(tags) == 0 {
		slog.Debug("tagging: no tags returned", "image_id", imageID)
		return nil
	}

	var tagIDs []model.PrimaryKey
	for _, name := range tags {
		tag, err := u.tagRepo.FindOrCreateTag(name)
		if err != nil {
			slog.Warn("tagging: failed to create tag", "tag", name, "err", err)
			continue
		}
		tagIDs = append(tagIDs, tag.ID)
	}

	if err := u.tagRepo.ReplaceImageTags(imageID, tagIDs); err != nil {
		return fmt.Errorf("store tags: %w", err)
	}

	slog.Debug("tagging: tagged", "image_id", imageID, "tags", tags)
	return nil
}

func (u *taggingUsecase) loadTaggingImage(imageID model.PrimaryKey) ([]byte, error) {
	img, err := u.imgRepo.FindById(imageID)
	if err != nil {
		return nil, fmt.Errorf("find image: %w", err)
	}

	if len(img.ThumbJPG) > 0 {
		return img.ThumbJPG, nil
	}

	if img.SrcType == "http" && len(img.ImageData) > 0 {
		thumb, err := makeThumbnail(img.ImageData)
		if err != nil {
			return nil, fmt.Errorf("generate thumbnail: %w", err)
		}
		return thumb, nil
	}

	return nil, fmt.Errorf("image %d has no thumbnail", imageID)
}

func (u *taggingUsecase) callTaggingService(ctx context.Context, endpoint string, imgBytes []byte, timeoutSec int) ([]string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	url := endpoint
	if maxTags := u.globalCfg.Tagging.MaxTags; maxTags > 0 {
		sep := "?"
		if bytes.Contains([]byte(url), []byte("?")) {
			sep = "&"
		}
		url = fmt.Sprintf("%s%smax_tags=%d", url, sep, maxTags)
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(imgBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "image/jpeg")

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // URL from config
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("call ai service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ai service returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Tags, nil
}

func makeThumbnail(imageData []byte) ([]byte, error) {
	img, err := decodeImageFromReader(bytes.NewReader(imageData))
	if err != nil {
		return nil, err
	}
	resized := imgconv.Resize(img, &imgconv.ResizeOption{Width: 256})
	buf := &bytes.Buffer{}
	if err := jpeg.Encode(buf, resized, &jpeg.Options{Quality: jpeg.DefaultQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeImageFromReader(r io.Reader) (image.Image, error) {
	img, _, err := image.Decode(r)
	return img, err
}
