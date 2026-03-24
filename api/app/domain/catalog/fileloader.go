package catalog

import (
	"bytes"
	"image"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrium/goheif"
	"github.com/mikyk10/wisp/app/domain/model"
	"github.com/rwcarlsen/goexif/exif"
)

// LoadImageFromPath loads an image and its metadata from a file path.
func LoadImageFromPath(path string) (image.Image, *model.ImgMeta, error) {
	return load(path)
}

func load(path string) (image.Image, *model.ImgMeta, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, nil, err
	}

	stat, _ := os.Stat(path)
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}

	defer file.Close()

	imgDecoder, exifDecoder := newImageDecoders(file)
	meta := &model.ImgMeta{}
	meta.ImageSourcePath = file.Name()
	meta.FileModifiedAt = stat.ModTime()

	exif, err := exifDecoder.DecodeExif()
	if err != nil {
		slog.Warn("no EXIF data", "path", path)
	}

	putExif(meta, exif)

	img, err := imgDecoder.DecodeImage()
	if err != nil {
		slog.Error("image decode failed", "path", path, "err", err)
		return nil, nil, err
	}

	return img, meta, nil
}

func loadMeta(path string) (*model.ImgMeta, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	stat, _ := os.Stat(path)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	_, exifDecoder := newImageDecoders(file)
	meta := &model.ImgMeta{
		ImageSourcePath: file.Name(),
		FileModifiedAt:  stat.ModTime(),
	}

	exifInfo, err := exifDecoder.DecodeExif()
	if err != nil {
		slog.Debug("no EXIF data (meta-only)", "path", path)
	}

	putExif(meta, exifInfo)

	return meta, nil
}

func newImageDecoders(file *os.File) (ImageDecoder, ExifDecoder) {
	var imgDec ImageDecoder
	var exifDec ExifDecoder

	ext := filepath.Ext(strings.ToLower(file.Name()))

	if ext == ".heic" {
		imgDec = &heicImageDecoder{file}
		exifDec = &heicExifDecoder{file}
	} else {
		imgDec = &jpegPngImageDecoder{file}
		exifDec = &jpegPngExifDecoder{file}
	}

	return imgDec, exifDec
}

type ImageDecoder interface {
	DecodeImage() (image.Image, error)
}
type ExifDecoder interface {
	DecodeExif() (*exif.Exif, error)
}

type heicImageDecoder struct {
	reader *os.File
}
type heicExifDecoder struct {
	reader *os.File
}

func (d *heicImageDecoder) DecodeImage() (image.Image, error) {
	d.reader.Seek(0, 0) //nolint:errcheck
	return goheif.Decode(d.reader)
}

func (d *heicExifDecoder) DecodeExif() (*exif.Exif, error) {
	d.reader.Seek(0, 0) //nolint:errcheck
	rawExif, err := goheif.ExtractExif(d.reader)
	if err != nil {
		return nil, err
	}

	exifInfo, err := exif.Decode(bytes.NewReader(rawExif))
	if err != nil {
		return nil, err
	}

	return exifInfo, nil
}

type jpegPngImageDecoder struct {
	reader *os.File
}
type jpegPngExifDecoder struct {
	reader *os.File
}

func (d *jpegPngImageDecoder) DecodeImage() (image.Image, error) {
	d.reader.Seek(0, 0) //nolint:errcheck
	img, _, err := image.Decode(d.reader)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func (d *jpegPngExifDecoder) DecodeExif() (*exif.Exif, error) {
	d.reader.Seek(0, 0) //nolint:errcheck
	exifInfo, err := exif.Decode(d.reader)
	if err != nil {
		return nil, err
	}
	return exifInfo, nil
}

func putExif(meta *model.ImgMeta, exif *exif.Exif) {

	if exif == nil {
		return
	}

	{
		tag, err := exif.Get("Orientation")
		if err != nil {
			slog.Warn("exif: failed to get orientation", "err", err)
		} else {
			val, err := tag.Int(0)
			if err != nil {
				slog.Warn("exif: failed to parse orientation value", "err", err)
			} else {
				meta.ExifOrientation = model.ExifOrientation(val)
			}
		}
	}

	if tag, err := exif.Get("SubjectArea"); err == nil {
		if x, ex := tag.Int(0); ex == nil {
			if y, ey := tag.Int(1); ey == nil {
				meta.ExifSubjectArea = image.Point{X: x, Y: y}
				meta.HasExifSubjectArea = true
				slog.Debug("exif: SubjectArea found", "x", x, "y", y, "count", tag.Count) //nolint:gosec // G706: values are parsed ints, not user input
			}
		}
	} else if tag, err := exif.Get("SubjectLocation"); err == nil {
		if x, ex := tag.Int(0); ex == nil {
			if y, ey := tag.Int(1); ey == nil {
				meta.ExifSubjectArea = image.Point{X: x, Y: y}
				meta.HasExifSubjectArea = true
				slog.Debug("exif: SubjectLocation found", "x", x, "y", y) //nolint:gosec // G706: values are parsed ints, not user input
			}
		}
	}

	takenAt, err := exif.DateTime()
	if err != nil {
		slog.Warn("exif: failed to get DateTime", "err", err)
	}

	//TODO: extract GPS information
	//exif.Get("GPSLatitude")
	//exif.Get("GPSLatitudeRef")
	//exif.Get("GPSLongitude")
	//exif.Get("GPSLongitudeRef")

	meta.ExifDateTime = takenAt
}
