package fs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFsImageFileFinder(t *testing.T) {
	dir, _ := os.Getwd()
	sut := NewFsImageFilePathFinder(fmt.Sprintf("%s/testdata/finder", dir))

	ch := make(chan string, 10)
	go sut.Find(context.Background(), ch)

	for f := range ch {
		basename := filepath.Base(f)
		assert.Contains(t, []string{"test1.png", "test2.png", "test3.png", "test3.jpg", "test4.png", "test4.jpeg", "test1.heic"}, basename)
		assert.NotContains(t, []string{"test1.txt", ".DS_Store"}, basename)
		assert.FileExists(t, f)
	}
}
