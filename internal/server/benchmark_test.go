package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MingToneFurry/Sensitive-lexicon/internal/config"
)

type benchOCR struct{}

func (benchOCR) Recognize(_ context.Context, _ []byte) (string, error) {
	return "这是坏词，这是测试词", nil
}

func (benchOCR) Enabled() bool { return true }

// minimalPNG generates a small 10×10 RGBA PNG image for use in benchmarks.
// Using a real PNG ensures the benchmark exercises actual image byte handling.
func minimalPNG(b *testing.B) []byte {
	b.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 25), G: uint8(y * 25), B: 100, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		b.Fatalf("encode test PNG: %v", err)
	}
	return buf.Bytes()
}

func BenchmarkDetectParallel(b *testing.B) {
	dir := b.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "words.txt"), []byte("坏词\n测试词\n"), 0o644); err != nil {
		b.Fatal(err)
	}
	s, err := New(config.Config{LexiconDir: dir, ReplaceSymbol: "*", BaseRPS: 1_000_000, AsyncQueueLength: 1024, MaxBodyBytes: 4096, BlockScoreThreshold: 0.1})
	if err != nil {
		b.Fatal(err)
	}
	h := s.middleware(http.HandlerFunc(s.detect))
	body := []byte(`{"text":"这是坏词，这是测试词"}`)

	b.ReportAllocs()
	b.SetParallelism(4)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodPost, "/detect", bytes.NewReader(body))
			res := httptest.NewRecorder()
			h.ServeHTTP(res, req)
			if res.Code != http.StatusOK {
				b.Fatalf("status=%d", res.Code)
			}
		}
	})
}

func BenchmarkDetectImageWithOCRParallel(b *testing.B) {
	dir := b.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "words.txt"), []byte("坏词\n测试词\n"), 0o644); err != nil {
		b.Fatal(err)
	}
	s, err := New(config.Config{LexiconDir: dir, ReplaceSymbol: "*", BaseRPS: 1_000_000, AsyncQueueLength: 1024, MaxBodyBytes: 2 << 20, BlockScoreThreshold: 0.1})
	if err != nil {
		b.Fatal(err)
	}
	s.ocr = benchOCR{}
	h := s.middleware(http.HandlerFunc(s.detectImage))

	// Use a real PNG image (10×10 pixels) so the benchmark exercises actual
	// image-byte dispatch, not just base64-encoding of an arbitrary string.
	imgBytes := minimalPNG(b)
	imgB64 := base64.StdEncoding.EncodeToString(imgBytes)
	body := []byte(`{"image_base64":"` + imgB64 + `"}`)

	b.ReportAllocs()
	b.SetParallelism(4)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodPost, "/detect/image", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			h.ServeHTTP(res, req)
			if res.Code != http.StatusOK {
				b.Fatalf("status=%d body=%s", res.Code, res.Body.String())
			}
		}
	})
}
