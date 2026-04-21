package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MingToneFurry/Sensitive-lexicon/internal/config"
)

func TestDetectUnauthorized(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("坏词\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := New(config.Config{LexiconDir: dir, ReplaceSymbol: "*", APIKey: "k", BaseRPS: 1000, AsyncQueueLength: 8, MaxBodyBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	h := s.middleware(http.HandlerFunc(s.detect))

	req := httptest.NewRequest(http.MethodPost, "/detect", bytes.NewBufferString(`{"text":"这是坏词"}`))
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.Code)
	}
}

func TestDetectWithValidAPIKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("坏词\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := New(config.Config{LexiconDir: dir, ReplaceSymbol: "*", APIKey: "k", BaseRPS: 1000, AsyncQueueLength: 8, MaxBodyBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	h := s.middleware(http.HandlerFunc(s.detect))

	req2 := httptest.NewRequest(http.MethodPost, "/detect", bytes.NewBufferString(`{"text":"这是坏词"}`))
	req2.Header.Set("X-API-Key", "k")
	res2 := httptest.NewRecorder()
	h.ServeHTTP(res2, req2)
	if res2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res2.Code)
	}
	if !strings.Contains(res2.Body.String(), `"contains":true`) {
		t.Fatalf("unexpected body: %s", res2.Body.String())
	}
}
