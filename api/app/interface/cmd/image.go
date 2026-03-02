package cmd

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
	"wspf/app/domain/catalog"
	"wspf/app/domain/display/epaper"
	"wspf/app/domain/encoder"
	"wspf/app/domain/improc"
	"wspf/app/domain/improc/color_reduction"
	"wspf/app/domain/improc/crop"
	"wspf/app/domain/improc/exif_rotation"
	"wspf/app/domain/improc/rotation"
	"wspf/app/domain/model"
	"wspf/app/domain/model/config"

	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

func NewImageConvertCommand(c *dig.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert <input-file>",
		Short: "Convert an image for a specific e-Paper display",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			displayModel, _ := cmd.Flags().GetString("display")
			orientationStr, _ := cmd.Flags().GetString("orientation")
			flip, _ := cmd.Flags().GetBool("flip")
			format, _ := cmd.Flags().GetString("format")
			outPath, _ := cmd.Flags().GetString("output")

			if !epaper.IsValidModel(epaper.EPaperDisplayModel(displayModel)) {
				return fmt.Errorf("unknown display model %q — valid models: %s", displayModel, strings.Join(epaper.ValidModels(), ", "))
			}

			if _, err := os.Stat(args[0]); err != nil {
				return fmt.Errorf("input file: %w", err)
			}

			orientation := config.NewDisplayOrientation(orientationStr)
			display := epaper.NewDisplay(
				epaper.EPaperDisplayModel(displayModel),
				model.CanonicalOrientation(orientation),
			)

			// Load image (EXIF metadata is extracted here for later rotation).
			imgLoader, err := catalog.NewFileImageLocator(args[0]).Resolve()
			if err != nil {
				return fmt.Errorf("failed to resolve image: %w", err)
			}
			img, meta, err := imgLoader.Load()
			if err != nil {
				return fmt.Errorf("failed to load image: %w", err)
			}

			// Build image processing pipeline.
			imseqGroup := improc.NewSequencerGroup()

			// Pre-processing: EXIF-based rotation, then crop to display dimensions.
			preSeq := improc.NewSequencer()
			imseqGroup.Push(preSeq)
			preSeq.Push(exif_rotation.NewExifRotation())
			preSeq.Push(crop.NewImageCropper(display))

			// Post-processing: color reduction, optional 180° flip.
			postSeq := improc.NewSequencer()
			imseqGroup.Push(postSeq)
			postSeq.Push(color_reduction.NewImageColorReduction(display, config.ColorReduction{Type: config.ColorReductionTypeFloydSteinberg}))
			if flip {
				postSeq.Push(rotation.NewRotation())
			}

			img, _ = imseqGroup.Apply(context.Background(), img, meta)

			// Determine output path.
			format = strings.ToLower(format)
			if outPath == "" {
				outPath = "out." + format
			}

			// Encode to the requested format.
			var buf *bytes.Buffer
			switch format {
			case "jpg", "jpeg":
				buf = &bytes.Buffer{}
				if err := jpeg.Encode(buf, img, nil); err != nil {
					return fmt.Errorf("jpeg encode: %w", err)
				}
			case "png":
				buf = &bytes.Buffer{}
				if err := png.Encode(buf, img); err != nil {
					return fmt.Errorf("png encode: %w", err)
				}
			default: // "bin" — Waveshare e-paper proprietary binary
				ecdr := encoder.NewWaveshareEPEncoder(display)
				buf, err = ecdr.Encode(img)
				if err != nil {
					return fmt.Errorf("waveshare encode: %w", err)
				}
			}

			if err := os.WriteFile(outPath, buf.Bytes(), 0644); err != nil { //nolint:gosec
				return fmt.Errorf("failed to write %s: %w", outPath, err)
			}

			fmt.Printf("wrote %s (%d bytes)\n", outPath, buf.Len())
			return nil
		},
	}

	cmd.Flags().StringP("display", "d", "", "Display model ("+strings.Join(epaper.ValidModels(), ", ")+")")
	_ = cmd.MarkFlagRequired("display")
	cmd.Flags().StringP("orientation", "O", "landscape", "Installed orientation: landscape, portrait")
	cmd.Flags().Bool("flip", false, "Rotate 180° (flip)")
	cmd.Flags().StringP("format", "f", "bin", "Output format: jpg, png, bin")
	cmd.Flags().StringP("output", "o", "", "Output file path (default: out.<format>)")

	return cmd
}
