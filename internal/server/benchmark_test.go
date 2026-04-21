package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MingToneFurry/Sensitive-lexicon/internal/config"
)

func BenchmarkDetectParallel(b *testing.B) {
	dir := b.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "words.txt"), []byte("坏词\n测试词\n"), 0o644); err != nil {
		b.Fatal(err)
	}
	s, err := New(config.Config{LexiconDir: dir, ReplaceSymbol: "*", BaseRPS: 1_000_000, AsyncQueueLength: 1024, MaxBodyBytes: 4096})
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
