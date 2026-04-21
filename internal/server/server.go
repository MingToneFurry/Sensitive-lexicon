package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MingToneFurry/Sensitive-lexicon/internal/config"
	"github.com/MingToneFurry/Sensitive-lexicon/internal/lexicon"
	"github.com/MingToneFurry/Sensitive-lexicon/internal/ratelimit"
)

type Server struct {
	cfg      config.Config
	engine   *lexicon.Engine
	limiter  *ratelimit.AdaptiveLimiter
	httpSrv  *http.Server
	jobs     chan asyncJob
	results  sync.Map
	idSeq    atomic.Uint64
	wordSize atomic.Int64
}

type asyncJob struct {
	id   string
	text string
}

type detectRequest struct {
	Text          string `json:"text"`
	ReplaceSymbol string `json:"replace_symbol,omitempty"`
}

type detectResponse struct {
	Contains bool   `json:"contains"`
	Replaced string `json:"replaced"`
	Count    int    `json:"count"`
}

func New(cfg config.Config) (*Server, error) {
	eng := lexicon.NewEngine(cfg.ReplaceSymbol, cfg.EnableBoundary)
	count, err := eng.LoadDir(cfg.LexiconDir)
	if err != nil {
		return nil, fmt.Errorf("load lexicon: %w", err)
	}
	s := &Server{
		cfg:     cfg,
		engine:  eng,
		limiter: ratelimit.New(cfg.BaseRPS),
		jobs:    make(chan asyncJob, cfg.AsyncQueueLength),
	}
	s.wordSize.Store(int64(count))
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/contains", s.contains)
	mux.HandleFunc("/detect", s.detect)
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
	jsonResponse(w, map[string]any{"status": "ok", "words": s.wordSize.Load()})
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
	matches := s.engine.Find(req.Text)
	jsonResponse(w, detectResponse{Contains: len(matches) > 0, Replaced: s.engine.ReplaceWithSymbol(req.Text, req.ReplaceSymbol), Count: len(matches)})
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
	case s.jobs <- asyncJob{id: id, text: req.Text}:
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
		matches := s.engine.Find(job.text)
		s.results.Store(job.id, detectResponse{Contains: len(matches) > 0, Replaced: s.engine.Replace(job.text), Count: len(matches)})
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
