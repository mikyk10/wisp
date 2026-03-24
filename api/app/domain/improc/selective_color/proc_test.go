package selective_color

import (
	"context"
	"image"
	"image/color"
	"testing"
)

func TestRgbToHSL(t *testing.T) {
	tests := []struct {
		name string
		r, g, b uint8
		wantH, wantS, wantL float64
		hTol, sTol, lTol    float64
	}{
		{"pure red", 255, 0, 0, 0, 1.0, 0.5, 1, 0.01, 0.01},
		{"pure green", 0, 255, 0, 120, 1.0, 0.5, 1, 0.01, 0.01},
		{"pure blue", 0, 0, 255, 240, 1.0, 0.5, 1, 0.01, 0.01},
		{"white", 255, 255, 255, 0, 0, 1.0, 1, 0.01, 0.01},
		{"black", 0, 0, 0, 0, 0, 0, 1, 0.01, 0.01},
		{"gray", 128, 128, 128, 0, 0, 0.502, 1, 0.01, 0.01},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, s, l := rgbToHSL(tt.r, tt.g, tt.b)
			if diff := abs(h - tt.wantH); diff > tt.hTol {
				t.Errorf("H = %f, want %f (diff %f)", h, tt.wantH, diff)
			}
			if diff := abs(s - tt.wantS); diff > tt.sTol {
				t.Errorf("S = %f, want %f (diff %f)", s, tt.wantS, diff)
			}
			if diff := abs(l - tt.wantL); diff > tt.lTol {
				t.Errorf("L = %f, want %f (diff %f)", l, tt.wantL, diff)
			}
		})
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestSelectiveColor_RedPreserved(t *testing.T) {
	p := NewSelectiveColor(map[string]string{
		"hue_center": "0",
		"hue_range":  "30",
	})

	img := image.NewRGBA(image.Rect(0, 0, 3, 1))
	// Red pixel (should stay color)
	img.SetRGBA(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	// Green pixel (should become gray)
	img.SetRGBA(1, 0, color.RGBA{R: 0, G: 255, B: 0, A: 255})
	// Gray pixel (should stay gray)
	img.SetRGBA(2, 0, color.RGBA{R: 128, G: 128, B: 128, A: 255})

	result, _ := p.Apply(context.Background(), img, nil)

	// Red pixel preserved
	r, g, b, _ := result.At(0, 0).RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 {
		t.Errorf("red pixel should be preserved, got (%d, %d, %d)", r>>8, g>>8, b>>8)
	}

	// Green pixel → grayscale
	r, g, b, _ = result.At(1, 0).RGBA()
	if r != g || g != b {
		t.Errorf("green pixel should be grayscale, got (%d, %d, %d)", r>>8, g>>8, b>>8)
	}

	// Gray pixel stays gray
	r, g, b, _ = result.At(2, 0).RGBA()
	if r != g || g != b {
		t.Errorf("gray pixel should remain grayscale, got (%d, %d, %d)", r>>8, g>>8, b>>8)
	}
}

func TestSelectiveColor_HueWrapAround(t *testing.T) {
	// Red hue wraps around 360/0 boundary
	p := NewSelectiveColor(map[string]string{
		"hue_center": "350",
		"hue_range":  "20",
	})

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	// Hue ~5 degrees (should be within range of 350±20 = 330-360,0-10)
	img.SetRGBA(0, 0, color.RGBA{R: 255, G: 21, B: 0, A: 255})

	result, _ := p.Apply(context.Background(), img, nil)

	r, g, b, _ := result.At(0, 0).RGBA()
	r8, g8, b8 := r>>8, g>>8, b>>8
	// Should preserve color (not grayscale)
	if r8 == g8 && g8 == b8 {
		t.Errorf("pixel near hue 5 should be preserved with center=350 range=20, got gray (%d)", r8)
	}
}

func TestSelectiveColor_DefaultValues(t *testing.T) {
	p, ok := NewSelectiveColor(map[string]string{}).(*processor)
	if !ok {
		t.Fatal("NewSelectiveColor did not return *processor")
	}
	if p.hueCenter != 0 {
		t.Errorf("default hue_center should be 0, got %f", p.hueCenter)
	}
	if p.hueRange != 30 {
		t.Errorf("default hue_range should be 30, got %f", p.hueRange)
	}
}
