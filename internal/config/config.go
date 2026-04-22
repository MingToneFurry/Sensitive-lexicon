package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
)

// Config holds all service configuration.
type Config struct {
	ListenAddr       string
	LexiconDir       string
	ReplaceSymbol    string
	EnableBoundary   bool
	APIKey           string
	BaseRPS          int
	MaxBodyBytes     int64
	AsyncQueueLength int
	// BlockScoreThreshold is the default sentence blocking threshold in [0,1],
	// where score=matchedRunes/totalRunes and blocked=true when score >= threshold.
	BlockScoreThreshold float64

	EnableOCR       bool
	OCRUseGPU       bool
	OCRGPUDevice    string
	OCRAutoDownload bool
	OCRRepoURL      string
	OCRModelDir     string
	OCRPythonBin    string
	OCRBridgeScript string
	OCRTimeoutSec   int
}

// fileConfig is the JSON-serializable representation used for the config file.
// Pointer types for booleans/numbers allow distinguishing "not set" from zero values.
type fileConfig struct {
	ListenAddr          string    `json:"listen_addr"`
	LexiconDir          string    `json:"lexicon_dir"`
	ReplaceSymbol       string    `json:"replace_symbol"`
	EnableBoundary      *bool     `json:"enable_boundary"`
	APIKey              string    `json:"api_key"`
	BaseRPS             *int      `json:"base_rps"`
	MaxBodyBytes        *int64    `json:"max_body_bytes"`
	AsyncQueueLength    *int      `json:"async_queue_length"`
	BlockScoreThreshold *float64  `json:"block_score_threshold"`
	OCR                 *ocrBlock `json:"ocr"`
}

type ocrBlock struct {
	Enable       *bool  `json:"enable"`
	UseGPU       *bool  `json:"use_gpu"`
	GPUDevice    string `json:"gpu_device"`
	AutoDownload *bool  `json:"auto_download"`
	RepoURL      string `json:"repo_url"`
	ModelDir     string `json:"model_dir"`
	PythonBin    string `json:"python_bin"`
	BridgeScript string `json:"bridge_script"`
	TimeoutSec   *int   `json:"timeout_sec"`
}

// defaults returns a Config populated with all built-in default values.
func defaults() Config {
	return Config{
		ListenAddr:          ":8080",
		LexiconDir:          "./Vocabulary",
		ReplaceSymbol:       "*",
		EnableBoundary:      true,
		BaseRPS:             600,
		MaxBodyBytes:        1 << 20,
		AsyncQueueLength:    128,
		BlockScoreThreshold: 0.2,
		OCRAutoDownload:     true,
		OCRRepoURL:          "https://github.com/DayBreak-u/chineseocr_lite.git",
		OCRModelDir:         "./ThirdPartyCompatibleFormats/chineseocr_lite",
		OCRPythonBin:        "python3",
		OCRBridgeScript:     "./internal/ocr/bridge.py",
		OCRTimeoutSec:       30,
		OCRGPUDevice:        "0",
	}
}

// Load reads configuration in priority order:
//  1. built-in defaults
//  2. JSON config file at `file` (if non-empty and the file exists)
//  3. environment variable overrides
//
// This means env vars always win over the file, preserving backward compatibility.
// If `file` is empty or the file does not exist, only defaults + env vars apply.
func Load(file string) Config {
	cfg := defaults()
	if file != "" {
		if err := loadFile(file, &cfg); err != nil {
			if !os.IsNotExist(err) {
				log.Printf("config: failed to load %s: %v — using defaults + env", file, err)
			}
		}
	}
	applyEnv(&cfg)
	return cfg
}

// LoadFromEnv loads config from environment variables only (backward-compatible).
func LoadFromEnv() Config {
	return Load("")
}

// loadFile parses the JSON config file and applies non-zero values to cfg.
func loadFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if fc.ListenAddr != "" {
		cfg.ListenAddr = fc.ListenAddr
	}
	if fc.LexiconDir != "" {
		cfg.LexiconDir = fc.LexiconDir
	}
	if fc.ReplaceSymbol != "" {
		cfg.ReplaceSymbol = fc.ReplaceSymbol
	}
	if fc.EnableBoundary != nil {
		cfg.EnableBoundary = *fc.EnableBoundary
	}
	if fc.APIKey != "" {
		cfg.APIKey = fc.APIKey
	}
	if fc.BaseRPS != nil {
		cfg.BaseRPS = *fc.BaseRPS
	}
	if fc.MaxBodyBytes != nil {
		cfg.MaxBodyBytes = *fc.MaxBodyBytes
	}
	if fc.AsyncQueueLength != nil {
		cfg.AsyncQueueLength = *fc.AsyncQueueLength
	}
	if fc.BlockScoreThreshold != nil {
		cfg.BlockScoreThreshold = *fc.BlockScoreThreshold
	}
	if o := fc.OCR; o != nil {
		if o.Enable != nil {
			cfg.EnableOCR = *o.Enable
		}
		if o.UseGPU != nil {
			cfg.OCRUseGPU = *o.UseGPU
		}
		if o.GPUDevice != "" {
			cfg.OCRGPUDevice = o.GPUDevice
		}
		if o.AutoDownload != nil {
			cfg.OCRAutoDownload = *o.AutoDownload
		}
		if o.RepoURL != "" {
			cfg.OCRRepoURL = o.RepoURL
		}
		if o.ModelDir != "" {
			cfg.OCRModelDir = o.ModelDir
		}
		if o.PythonBin != "" {
			cfg.OCRPythonBin = o.PythonBin
		}
		if o.BridgeScript != "" {
			cfg.OCRBridgeScript = o.BridgeScript
		}
		if o.TimeoutSec != nil {
			cfg.OCRTimeoutSec = *o.TimeoutSec
		}
	}
	return nil
}

// applyEnv overlays environment-variable values onto cfg. Only non-empty
// environment variables are applied, so unset variables leave the file / default
// values unchanged.
func applyEnv(cfg *Config) {
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("LEXICON_DIR"); v != "" {
		cfg.LexiconDir = v
	}
	if v := os.Getenv("REPLACE_SYMBOL"); v != "" {
		cfg.ReplaceSymbol = v
	}
	if v := os.Getenv("ENABLE_BOUNDARY"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.EnableBoundary = b
		}
	}
	if v := os.Getenv("API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("BASE_RPS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.BaseRPS = n
		}
	}
	if v := os.Getenv("MAX_BODY_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.MaxBodyBytes = n
		}
	}
	if v := os.Getenv("ASYNC_QUEUE_LENGTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.AsyncQueueLength = n
		}
	}
	if v := os.Getenv("BLOCK_SCORE_THRESHOLD"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.BlockScoreThreshold = n
		}
	}
	if v := os.Getenv("ENABLE_OCR"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.EnableOCR = b
		}
	}
	if v := os.Getenv("OCR_USE_GPU"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.OCRUseGPU = b
		}
	}
	if v := os.Getenv("OCR_GPU_DEVICE"); v != "" {
		cfg.OCRGPUDevice = v
	}
	if v := os.Getenv("OCR_AUTO_DOWNLOAD"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.OCRAutoDownload = b
		}
	}
	if v := os.Getenv("OCR_REPO_URL"); v != "" {
		cfg.OCRRepoURL = v
	}
	if v := os.Getenv("OCR_MODEL_DIR"); v != "" {
		cfg.OCRModelDir = v
	}
	if v := os.Getenv("OCR_PYTHON_BIN"); v != "" {
		cfg.OCRPythonBin = v
	}
	if v := os.Getenv("OCR_BRIDGE_SCRIPT"); v != "" {
		cfg.OCRBridgeScript = v
	}
	if v := os.Getenv("OCR_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.OCRTimeoutSec = n
		}
	}
}
