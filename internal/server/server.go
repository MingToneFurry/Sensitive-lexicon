package server

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/MingToneFurry/Sensitive-lexicon/internal/config"
	"github.com/MingToneFurry/Sensitive-lexicon/internal/lexicon"
	"github.com/MingToneFurry/Sensitive-lexicon/internal/ocr"
	"github.com/MingToneFurry/Sensitive-lexicon/internal/ratelimit"
)

type Server struct {
	cfg      config.Config
	engine   *lexicon.Engine
	ocr      ocr.Client
	limiter  *ratelimit.AdaptiveLimiter
	httpSrv  *http.Server
	jobs     chan asyncJob
	results  sync.Map
	idSeq    atomic.Uint64
	wordSize atomic.Int64
}

type asyncJob struct {
	id            string
	text          string
	replaceSymbol string
	threshold     *float64
}

type detectRequest struct {
	Text           string   `json:"text,omitempty"`
	ReplaceSymbol  string   `json:"replace_symbol,omitempty"`
	BlockThreshold *float64 `json:"block_threshold,omitempty"`
}

type detectImageRequest struct {
	ImageBase64    string   `json:"image_base64,omitempty"`
	ReplaceSymbol  string   `json:"replace_symbol,omitempty"`
	BlockThreshold *float64 `json:"block_threshold,omitempty"`
}

type detectResponse struct {
	Contains  bool    `json:"contains"`
	Replaced  string  `json:"replaced"`
	Count     int     `json:"count"`
	Score     float64 `json:"score"`
	Blocked   bool    `json:"blocked"`
	Threshold float64 `json:"threshold"`
}

type detectImageResponse struct {
	detectResponse
	OCRText string `json:"ocr_text"`
}

const scoreRoundScale = 10000.0

func New(cfg config.Config) (*Server, error) {
	eng := lexicon.NewEngine(cfg.ReplaceSymbol, cfg.EnableBoundary)
	count, err := eng.LoadDir(cfg.LexiconDir)
	if err != nil {
		return nil, fmt.Errorf("load lexicon: %w", err)
	}
	ocrClient, err := ocr.New(ocr.Settings{
		Enable:       cfg.EnableOCR,
		UseGPU:       cfg.OCRUseGPU,
		GPUDevice:    cfg.OCRGPUDevice,
		AutoDownload: cfg.OCRAutoDownload,
		RepoURL:      cfg.OCRRepoURL,
		ModelRepoDir: cfg.OCRModelDir,
		PythonBin:    cfg.OCRPythonBin,
		BridgeScript: cfg.OCRBridgeScript,
		TimeoutSec:   cfg.OCRTimeoutSec,
	})
	if err != nil {
		return nil, fmt.Errorf("init ocr: %w", err)
	}
	s := &Server{
		cfg:     cfg,
		engine:  eng,
		ocr:     ocrClient,
		limiter: ratelimit.New(cfg.BaseRPS),
		jobs:    make(chan asyncJob, cfg.AsyncQueueLength),
	}
	s.wordSize.Store(int64(count))
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/contains", s.contains)
	mux.HandleFunc("/detect", s.detect)
	mux.HandleFunc("/detect/image", s.detectImage)
	mux.HandleFunc("/reload", s.reload)
	mux.HandleFunc("/detect/async", s.detectAsync)
	mux.HandleFunc("/detect/async/result", s.detectAsyncResult)
	mux.HandleFunc("/sanitize-stream", s.sanitizeStream)
	s.httpSrv = &http.Server{Addr: cfg.ListenAddr, Handler: s.middleware(mux), ReadHeaderTimeout: 3 * time.Second}
	for i := 0; i < 2; i++ {
		go s.worker()
	}
	return s, nil
}

func (s *Server) ListenAndServe() error { return s.httpSrv.ListenAndServe() }

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.Allow() {
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		if s.cfg.APIKey != "" && r.URL.Path != "/health" {
			if r.Header.Get("X-API-Key") != s.cfg.APIKey {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	jsonResponse(w, map[string]any{"status": "ok", "words": s.wordSize.Load(), "ocr_enabled": s.ocr.Enabled()})
}

func (s *Server) contains(w http.ResponseWriter, r *http.Request) {
	text := r.URL.Query().Get("text")
	jsonResponse(w, map[string]bool{"contains": s.engine.Contains(text)})
}

func (s *Server) detect(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)
	defer r.Body.Close()
	var req detectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	jsonResponse(w, s.analyzeText(req.Text, req.ReplaceSymbol, req.BlockThreshold))
}

func (s *Server) detectImage(w http.ResponseWriter, r *http.Request) {
	if !s.ocr.Enabled() {
		http.Error(w, "ocr not enabled", http.StatusServiceUnavailable)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)
	defer r.Body.Close()

	imgBytes, replaceSymbol, threshold, err := s.readImageRequest(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	text, err := s.ocr.Recognize(r.Context(), imgBytes)
	if err != nil {
		var clientErr *ocr.InvalidInputError
		if errors.As(err, &clientErr) {
			http.Error(w, fmt.Sprintf("invalid image: %v", err), http.StatusBadRequest)
			return
		}
		http.Error(w, fmt.Sprintf("ocr failed: %v", err), http.StatusInternalServerError)
		return
	}
	resp := detectImageResponse{detectResponse: s.analyzeText(text, replaceSymbol, threshold), OCRText: text}
	jsonResponse(w, resp)
}

func (s *Server) readImageRequest(w http.ResponseWriter, r *http.Request) ([]byte, string, *float64, error) {
	contentType := r.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(contentType)

	switch {
	case strings.HasPrefix(mediaType, "application/json"):
		var req detectImageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, "", nil, fmt.Errorf("invalid json")
		}
		img, err := decodeBase64Image(req.ImageBase64)
		if err != nil {
			return nil, "", nil, err
		}
		return img, req.ReplaceSymbol, req.BlockThreshold, nil
	case strings.HasPrefix(mediaType, "multipart/form-data"):
		if err := r.ParseMultipartForm(s.cfg.MaxBodyBytes); err != nil {
			return nil, "", nil, fmt.Errorf("invalid multipart form")
		}
		file, _, err := r.FormFile("image")
		if err != nil {
			return nil, "", nil, fmt.Errorf("image file is required")
		}
		defer file.Close()
		img, err := io.ReadAll(file)
		if err != nil || len(img) == 0 {
			return nil, "", nil, fmt.Errorf("invalid image")
		}
		if int64(len(img)) > s.cfg.MaxBodyBytes {
			return nil, "", nil, fmt.Errorf("image too large")
		}
		replaceSymbol := r.FormValue("replace_symbol")
		threshold := parseThreshold(r.FormValue("block_threshold"))
		return img, replaceSymbol, threshold, nil
	default:
		img, err := io.ReadAll(r.Body)
		if err != nil || len(img) == 0 {
			return nil, "", nil, fmt.Errorf("empty request body")
		}
		return img, "", nil, nil
	}
}

func parseThreshold(v string) *float64 {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	var threshold float64
	if _, err := fmt.Sscanf(v, "%f", &threshold); err != nil {
		return nil
	}
	return &threshold
}

func decodeBase64Image(raw string) ([]byte, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return nil, fmt.Errorf("image_base64 is required")
	}
	if idx := strings.Index(v, ","); idx >= 0 && strings.Contains(v[:idx], "base64") {
		v = v[idx+1:]
	}
	img, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		img, err = base64.RawStdEncoding.DecodeString(v)
	}
	if err != nil || len(img) == 0 {
		return nil, fmt.Errorf("invalid base64 image")
	}
	return img, nil
}

func (s *Server) analyzeText(text, replaceSymbol string, reqThreshold *float64) detectResponse {
	matches := s.engine.Find(text)
	totalRunes := utf8.RuneCountInString(text)
	matchedRunes := 0
	for _, m := range matches {
		if m.End > m.Start {
			matchedRunes += (m.End - m.Start)
		}
	}
	score := 0.0
	if totalRunes > 0 {
		score = float64(matchedRunes) / float64(totalRunes)
	}
	threshold := s.cfg.BlockScoreThreshold
	if reqThreshold != nil {
		threshold = *reqThreshold
	}
	threshold = clampThreshold(threshold)
	score = math.Round(score*scoreRoundScale) / scoreRoundScale
	threshold = math.Round(threshold*scoreRoundScale) / scoreRoundScale
	contains := len(matches) > 0
	blocked := contains && score >= threshold
	return detectResponse{
		Contains:  contains,
		Replaced:  s.engine.ReplaceWithMatches(text, replaceSymbol, matches),
		Count:     len(matches),
		Score:     score,
		Blocked:   blocked,
		Threshold: threshold,
	}
}

func clampThreshold(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func (s *Server) reload(w http.ResponseWriter, _ *http.Request) {
	count, err := s.engine.LoadDir(s.cfg.LexiconDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.wordSize.Store(int64(count))
	jsonResponse(w, map[string]any{"reloaded": true, "words": count})
}

func (s *Server) detectAsync(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)
	defer r.Body.Close()
	var req detectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	id := fmt.Sprintf("job-%d", s.idSeq.Add(1))
	select {
	case s.jobs <- asyncJob{id: id, text: req.Text, replaceSymbol: req.ReplaceSymbol, threshold: req.BlockThreshold}:
		jsonResponse(w, map[string]string{"job_id": id})
	default:
		http.Error(w, "queue full", http.StatusTooManyRequests)
	}
}

func (s *Server) detectAsyncResult(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("job_id")
	if id == "" {
		http.Error(w, "job_id required", http.StatusBadRequest)
		return
	}
	if v, ok := s.results.Load(id); ok {
		jsonResponse(w, v)
		return
	}
	http.Error(w, "pending", http.StatusAccepted)
}

func (s *Server) worker() {
	for job := range s.jobs {
		s.results.Store(job.id, s.analyzeText(job.text, job.replaceSymbol, job.threshold))
	}
}

func (s *Server) sanitizeStream(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)
	defer r.Body.Close()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	reader := bufio.NewReader(r.Body)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			if _, writeErr := io.WriteString(w, s.engine.Replace(strings.TrimSuffix(line, "\n"))+"\n"); writeErr != nil {
				log.Printf("stream write failed: %v", writeErr)
				return
			}
		}
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Printf("stream read failed: %v", err)
			http.Error(w, "stream processing error", http.StatusBadRequest)
			return
		}
	}
}

func jsonResponse(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write json response: %v", err)
	}
}
