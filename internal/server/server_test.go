package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MingToneFurry/Sensitive-lexicon/internal/config"
	"github.com/MingToneFurry/Sensitive-lexicon/internal/ocr"
)

type stubOCR struct {
	text string
	err  error
}

func (s stubOCR) Recognize(_ context.Context, _ []byte) (string, error) { return s.text, s.err }
func (s stubOCR) Enabled() bool                                         { return true }

func testConfig(dir, apiKey string) config.Config {
	return config.Config{
		LexiconDir:          dir,
		ReplaceSymbol:       "*",
		APIKey:              apiKey,
		BaseRPS:             1000,
		AsyncQueueLength:    8,
		MaxBodyBytes:        1 << 20,
		BlockScoreThreshold: 0.1,
	}
}

func newTestServer(t *testing.T, apiKey string) *Server {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("坏词\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := New(testConfig(dir, apiKey))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDetectUnauthorized(t *testing.T) {
	s := newTestServer(t, "k")
	h := s.middleware(http.HandlerFunc(s.detect))

	req := httptest.NewRequest(http.MethodPost, "/detect", bytes.NewBufferString(`{"text":"这是坏词"}`))
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.Code)
	}
}

func TestDetectWithValidAPIKey(t *testing.T) {
	s := newTestServer(t, "k")
	h := s.middleware(http.HandlerFunc(s.detect))

	req := httptest.NewRequest(http.MethodPost, "/detect", bytes.NewBufferString(`{"text":"这是坏词"}`))
	req.Header.Set("X-API-Key", "k")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), `"contains":true`) {
		t.Fatalf("unexpected body: %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"blocked":true`) {
		t.Fatalf("expected blocked true: %s", res.Body.String())
	}
}

func TestDetectThresholdOverride(t *testing.T) {
	s := newTestServer(t, "")
	h := s.middleware(http.HandlerFunc(s.detect))

	req := httptest.NewRequest(http.MethodPost, "/detect", bytes.NewBufferString(`{"text":"这是坏词","block_threshold":0.9}`))
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if strings.Contains(res.Body.String(), `"blocked":true`) {
		t.Fatalf("expected blocked false with high threshold: %s", res.Body.String())
	}
}

func TestDetectImageDisabled(t *testing.T) {
	s := newTestServer(t, "")
	h := s.middleware(http.HandlerFunc(s.detectImage))

	req := httptest.NewRequest(http.MethodPost, "/detect/image", bytes.NewBufferString(`{"image_base64":"aGVsbG8="}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", res.Code)
	}
}

func TestDetectImageWithStubOCR(t *testing.T) {
	s := newTestServer(t, "")
	s.ocr = stubOCR{text: "图片里有坏词"}
	h := s.middleware(http.HandlerFunc(s.detectImage))

	imgB64 := base64.StdEncoding.EncodeToString([]byte("fake-image"))
	reqBody := map[string]any{"image_base64": imgB64}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/detect/image", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"ocr_text":"图片里有坏词"`) {
		t.Fatalf("unexpected body: %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"contains":true`) {
		t.Fatalf("unexpected body: %s", res.Body.String())
	}
}

func TestDetectImageInvalidInputReturns400(t *testing.T) {
	s := newTestServer(t, "")
	s.ocr = stubOCR{err: &ocr.InvalidInputError{Msg: "cannot identify image file"}}
	h := s.middleware(http.HandlerFunc(s.detectImage))

	imgB64 := base64.StdEncoding.EncodeToString([]byte("not-an-image"))
	reqBody := map[string]any{"image_base64": imgB64}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/detect/image", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestDetectImageServerErrorReturns500(t *testing.T) {
	s := newTestServer(t, "")
	s.ocr = stubOCR{err: fmt.Errorf("ocr subprocess crashed")}
	h := s.middleware(http.HandlerFunc(s.detectImage))

	imgB64 := base64.StdEncoding.EncodeToString([]byte("some-image"))
	reqBody := map[string]any{"image_base64": imgB64}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/detect/image", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", res.Code, res.Body.String())
	}
}
