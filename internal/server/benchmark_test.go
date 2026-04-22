package server

import (
	"bytes"
	"context"
	"encoding/base64"
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
	imgB64 := base64.StdEncoding.EncodeToString([]byte("fake-image-content"))
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
