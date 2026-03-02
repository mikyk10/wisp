package catalog

import (
	"context"
	"fmt"
	"testing"
	"github.com/mikyk10/wisp/app/domain/model/config"
)

func TestEnumerateImages(t *testing.T) {

	foundCh := make(chan ImageLoader, 1000)
	excludedCh := make(chan ImageLoader, 1000)

	var imfc = &imageIndexedFileProvider{
		fileProviderConfig: config.ImageFileProviderConfig{
			SrcPath: "/tmp/testdata/pictures",
			Criteria: config.Criteria{
				Include: config.FileCriteria{
					//Path: []string{"Pictures"},
					//ExifTimeRange: []config.TimeRange{
					//{From: time.Date(2025, 3, 1, 0, 0, 0, 0, time.Local), To: time.Date(2025, 3, 4, 20, 4, 0, 0, time.Local)},
					//},
				},
				Exclude: config.FileCriteria{
					//ExifTimeRange: []config.TimeRange{
					//	{From: time.Date(2025, 3, 1, 0, 0, 0, 0, time.Local), To: time.Date(2025, 3, 4, 20, 1, 0, 0, time.Local)},
					//},
				},
			},
		},
	}

	ctx := context.Background()
	go imfc.EnumerateImages(ctx, foundCh, excludedCh)

	for {
		select {
		case <-ctx.Done():
			fmt.Println("aaa done")
			return

		case _, ok := <-foundCh:
			if !ok {
				foundCh = nil
				break
			}

		case _, ok := <-excludedCh:
			if !ok {
				excludedCh = nil
				break
			}

		default:
			if foundCh == nil && excludedCh == nil {
				fmt.Println("done")
				return
			}
		}
	}
}
