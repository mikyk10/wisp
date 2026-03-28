// testserver is a minimal HTTP image server for testing the HTTP catalog fetch.
//
// Endpoints:
//   GET  /image       → returns a randomly colored 800x600 JPEG
//   POST /remix       → reads posted image, returns a tinted version as JPEG
//   GET  /health      → 200 OK
//
// Usage:
//   go run ./cmd/testserver
//   go run ./cmd/testserver -addr :9090
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"time"
)

func main() {
	addr := flag.String("addr", ":9090", "listen address")
	flag.Parse()

	http.HandleFunc("/image", handleImage)
	http.HandleFunc("/remix", handleRemix)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	log.Printf("testserver listening on %s", *addr)
	log.Printf("  GET  /image  → random color image")
	log.Printf("  POST /remix  → tinted remix of posted image")
	srv := &http.Server{
		Addr:         *addr,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

// handleImage generates a random solid-color 800x600 JPEG.
func handleImage(w http.ResponseWriter, r *http.Request) {
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	c := color.RGBA{
		R: uint8(rand.IntN(256)), //nolint:gosec // 0-255 always fits uint8
		G: uint8(rand.IntN(256)), //nolint:gosec // 0-255 always fits uint8
		B: uint8(rand.IntN(256)), //nolint:gosec // 0-255 always fits uint8
		A: 255,
	}
	draw.Draw(img, img.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)

	// Draw a simple grid pattern for visual verification.
	for x := range 800 {
		for y := range 600 {
			if x%100 == 0 || y%100 == 0 {
				img.Set(x, y, color.RGBA{255, 255, 255, 80})
			}
		}
	}

	w.Header().Set("Content-Type", "image/jpeg")
	jpeg.Encode(w, img, &jpeg.Options{Quality: 85}) //nolint:errcheck
	log.Printf("GET /image → %v", c)
}

// handleRemix reads a posted image, applies a color tint, and returns it.
func handleRemix(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	src, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		http.Error(w, fmt.Sprintf("decode failed: %v", err), http.StatusBadRequest)
		return
	}

	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)

	// Apply a random color tint.
	tintR := uint8(rand.IntN(128)) //nolint:gosec // 0-127 always fits uint8
	tintG := uint8(rand.IntN(128)) //nolint:gosec // 0-127 always fits uint8
	tintB := uint8(rand.IntN(128)) //nolint:gosec // 0-127 always fits uint8

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := src.At(x, y).RGBA()
			dst.Set(x, y, color.RGBA{
				R: uint8(r>>8)/2 + tintR,   //nolint:gosec // r>>8 is 0-255
				G: uint8(g>>8)/2 + tintG,   //nolint:gosec // g>>8 is 0-255
				B: uint8(b>>8)/2 + tintB,   //nolint:gosec // b>>8 is 0-255
				A: uint8(a >> 8),            //nolint:gosec // a>>8 is 0-255
			})
		}
	}

	w.Header().Set("Content-Type", "image/jpeg")
	jpeg.Encode(w, dst, &jpeg.Options{Quality: 85}) //nolint:errcheck
	log.Printf("POST /remix → tint(%d,%d,%d) size=%dx%d input=%d bytes", tintR, tintG, tintB, bounds.Dx(), bounds.Dy(), len(body))
}
