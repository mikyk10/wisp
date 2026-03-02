package fs

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"github.com/mikyk10/wisp/app/domain/finder"
)

func NewFsImageFilePathFinder(dirnames ...string) finder.PathStreamFinder {
	return &fsImageFileFinder{
		paths: dirnames,
	}
}

type fsImageFileFinder struct {
	paths []string
}

func (f *fsImageFileFinder) Find(ctx context.Context, resultChan chan<- string) {
	defer close(resultChan)

	for _, path := range f.paths {
		err := filepath.WalkDir(path,
			func(path string, de fs.DirEntry, err error) error {

				select {
				case <-ctx.Done():
					return filepath.SkipAll
				default:
				}

				if err != nil {
					if !errors.Is(err, fs.ErrNotExist) {
						slog.Warn("walk: directory entry error", "err", err)
					}
					return err
				}
				if de.IsDir() {
					return nil
				}

				ext := filepath.Ext(strings.ToLower(path))

				switch ext {
				case ".jpeg":
					fallthrough
				case ".jpg":
					fallthrough
				case ".png":
					fallthrough
				case ".heic":
					resultChan <- path
				}

				return nil
			})

		if err != nil {
			log.Fatal(err)
		}
	}
}

func NewConfigFilePathFinder(dirname ...string) finder.PathFinder {
	return &fsConfigFileFinder{
		paths: dirname,
	}
}

type fsConfigFileFinder struct {
	paths []string
}

func (f *fsConfigFileFinder) Find(basename ...string) string {

	cwd, _ := os.Getwd()
	for _, path := range f.paths {

		if strings.HasPrefix(path, "./") {
			path = path[2:]
			path = filepath.Join(cwd, path)
		}

		var found *string
		_ = filepath.WalkDir(path,

			func(path string, de fs.DirEntry, err error) error {
				if err != nil {
					if !errors.Is(err, fs.ErrNotExist) {
						slog.Warn("config search: directory entry error", "err", err)
					}
					return err
				}
				if de.IsDir() {
					return nil
				}

				ext := filepath.Ext(strings.ToLower(path))
				bn := filepath.Base(path)

				if (ext == ".yaml" || ext == ".yml") && slices.Contains(basename, bn) {
					found = &path
					return fs.SkipAll
				}

				return nil
			})

		if found != nil {
			return *found
		}
	}

	log.Fatal("config file not found in search paths")
	return "" // unreachable
}
