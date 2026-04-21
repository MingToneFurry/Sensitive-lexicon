package main

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MingToneFurry/Sensitive-lexicon/internal/lexicon"
)

type config struct {
	Port         string
	LexiconDir   string
	APIKey       string
	Replace      string
	KeepLength   bool
	Workers      int
	QueueSize    int
	BaseRPS      int64
	MinRPS       int64
	MaxRPS       int64
	AdaptSeconds int
}

type detector struct {
	trie  atomic.Pointer[lexicon.Trie]
	count atomic.Int64
	dir   string
}

func (d *detector) reload() error {
	trie, count, err := lexicon.LoadFromDir(d.dir)
	if err != nil {
		return err
	}
	d.trie.Store(trie)
	d.count.Store(int64(count))
	return nil
}

type detectRequest struct {
	Text       string `json:"text"`
	Replace    bool   `json:"replace"`
	Symbol     string `json:"symbol,omitempty"`
	KeepLength *bool  `json:"keep_length,omitempty"`
}

type detectResponse struct {
	Contains bool            `json:"contains"`
	Matches  []lexicon.Match `json:"matches,omitempty"`
	Masked   string          `json:"masked,omitempty"`
}

type asyncJob struct {
	ID  string
	Req detectRequest
}

type asyncResult struct {
	Done      bool           `json:"done"`
	Error     string         `json:"error,omitempty"`
	Response  detectResponse `json:"response,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

func loadConfig() config {
	return config{
		Port:         getenv("PORT", "8080"),
		LexiconDir:   getenv("LEXICON_DIR", "./Vocabulary"),
		APIKey:       os.Getenv("API_KEY"),
		Replace:      getenv("REPLACE_SYMBOL", "*"),
		KeepLength:   getenv("KEEP_REPLACEMENT_LENGTH", "true") == "true",
		Workers:      getenvInt("ASYNC_WORKERS", runtime.NumCPU()),
		QueueSize:    getenvInt("ASYNC_QUEUE_SIZE", 1024),
		BaseRPS:      int64(getenvInt("BASE_RPS", 300)),
		MinRPS:       int64(getenvInt("MIN_RPS", 50)),
		MaxRPS:       int64(getenvInt("MAX_RPS", 1500)),
		AdaptSeconds: getenvInt("ADAPT_INTERVAL_SECONDS", 2),
	}
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func main() {
	cfg := loadConfig()
	d := &detector{dir: cfg.LexiconDir}
	if err := d.reload(); err != nil {
		log.Fatalf("failed to load lexicon: %v", err)
	}

	limiter := lexicon.NewAdaptiveLimiter(cfg.BaseRPS, cfg.MinRPS, cfg.MaxRPS, time.Duration(cfg.AdaptSeconds)*time.Second)
	defer limiter.Close()

	queue := make(chan asyncJob, cfg.QueueSize)
	var results sync.Map
	for i := 0; i < cfg.Workers; i++ {
		go func() {
			for job := range queue {
				resp, err := runDetect(d, cfg, job.Req)
				value, ok := results.Load(job.ID)
				if !ok {
					continue
				}
				state := value.(asyncResult)
				state.Done = true
				if err != nil {
					state.Error = err.Error()
				} else {
					state.Response = resp
				}
				results.Store(job.ID, state)
			}
		}()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "lexicon_words": d.count.Load(), "rps": limiter.CurrentRPS()})
	})
	mux.HandleFunc("/reload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := d.reload(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"reloaded": true, "lexicon_words": d.count.Load()})
	})
	mux.HandleFunc("/detect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !limiter.Allow() {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		var req detectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
		resp, err := runDetect(d, cfg, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
	mux.HandleFunc("/contains", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !limiter.Allow() {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		var req detectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
		if strings.TrimSpace(req.Text) == "" {
			writeError(w, http.StatusBadRequest, "text is required")
			return
		}
		trie := d.trie.Load()
		matches := trie.FindMatches(req.Text)
		writeJSON(w, http.StatusOK, map[string]bool{"contains": len(matches) > 0})
	})
	mux.HandleFunc("/detect/async", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !limiter.Allow() {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		var req detectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
		if strings.TrimSpace(req.Text) == "" {
			writeError(w, http.StatusBadRequest, "text is required")
			return
		}
		id := makeID()
		results.Store(id, asyncResult{Done: false, CreatedAt: time.Now()})
		select {
		case queue <- asyncJob{ID: id, Req: req}:
			writeJSON(w, http.StatusAccepted, map[string]string{"job_id": id})
		default:
			results.Delete(id)
			writeError(w, http.StatusServiceUnavailable, "async queue is full")
		}
	})
	mux.HandleFunc("/detect/result", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			writeError(w, http.StatusBadRequest, "id is required")
			return
		}
		value, ok := results.Load(id)
		if !ok {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		res := value.(asyncResult)
		if time.Since(res.CreatedAt) > 10*time.Minute {
			results.Delete(id)
		}
		writeJSON(w, http.StatusOK, res)
	})
	mux.HandleFunc("/detect/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !limiter.Allow() {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "stream unsupported")
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		scanner := bufio.NewScanner(r.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			resp, err := runDetect(d, cfg, detectRequest{Text: line, Replace: true})
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			_, _ = w.Write([]byte(resp.Masked + "\n"))
			flusher.Flush()
		}
		if err := scanner.Err(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	})

	handler := withAPIKey(cfg.APIKey, mux)
	server := &http.Server{Addr: ":" + cfg.Port, Handler: handler, ReadHeaderTimeout: 5 * time.Second}

	log.Printf("server started on :%s, words=%d", cfg.Port, d.count.Load())
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	_ = server.Shutdown(context.Background())
}

func runDetect(d *detector, cfg config, req detectRequest) (detectResponse, error) {
	if strings.TrimSpace(req.Text) == "" {
		return detectResponse{}, errors.New("text is required")
	}
	trie := d.trie.Load()
	matches := trie.FindMatches(req.Text)
	resp := detectResponse{Contains: len(matches) > 0, Matches: matches}
	if req.Replace {
		symbol := cfg.Replace
		if strings.TrimSpace(req.Symbol) != "" {
			symbol = req.Symbol
		}
		keepLength := cfg.KeepLength
		if req.KeepLength != nil {
			keepLength = *req.KeepLength
		}
		resp.Masked = lexicon.Replace(req.Text, matches, symbol, keepLength)
	}
	return resp, nil
}

func withAPIKey(apiKey string, next http.Handler) http.Handler {
	if strings.TrimSpace(apiKey) == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		key := strings.TrimSpace(r.Header.Get("X-API-Key"))
		if subtle.ConstantTimeCompare([]byte(key), []byte(apiKey)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid api key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func makeID() string {
	return fmt.Sprintf("%d-%06d", time.Now().UnixNano(), rand.Intn(1000000))
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
