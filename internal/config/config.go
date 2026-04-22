package config

import (
	"os"
	"strconv"
)

type Config struct {
	ListenAddr       string
	LexiconDir       string
	ReplaceSymbol    string
	EnableBoundary   bool
	APIKey           string
	BaseRPS          int
	MaxBodyBytes     int64
	AsyncQueueLength int
	// BlockScoreThreshold is the default sentence blocking threshold in [0,1], where score=matchedRunes/totalRunes and blocked=true when score >= threshold.
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

func LoadFromEnv() Config {
	return Config{
		ListenAddr:          getEnv("LISTEN_ADDR", ":8080"),
		LexiconDir:          getEnv("LEXICON_DIR", "./Vocabulary"),
		ReplaceSymbol:       getEnv("REPLACE_SYMBOL", "*"),
		EnableBoundary:      getEnvBool("ENABLE_BOUNDARY", true),
		APIKey:              os.Getenv("API_KEY"),
		BaseRPS:             getEnvInt("BASE_RPS", 600),
		MaxBodyBytes:        getEnvInt64("MAX_BODY_BYTES", 1<<20),
		AsyncQueueLength:    getEnvInt("ASYNC_QUEUE_LENGTH", 128),
		BlockScoreThreshold: getEnvFloat64("BLOCK_SCORE_THRESHOLD", 0.2),

		EnableOCR:       getEnvBool("ENABLE_OCR", false),
		OCRUseGPU:       getEnvBool("OCR_USE_GPU", false),
		OCRGPUDevice:    getEnv("OCR_GPU_DEVICE", "0"),
		OCRAutoDownload: getEnvBool("OCR_AUTO_DOWNLOAD", true),
		OCRRepoURL:      getEnv("OCR_REPO_URL", "https://github.com/DayBreak-u/chineseocr_lite.git"),
		OCRModelDir:     getEnv("OCR_MODEL_DIR", "./ThirdPartyCompatibleFormats/chineseocr_lite"),
		OCRPythonBin:    getEnv("OCR_PYTHON_BIN", "python3"),
		OCRBridgeScript: getEnv("OCR_BRIDGE_SCRIPT", "./internal/ocr/bridge.py"),
		OCRTimeoutSec:   getEnvInt("OCR_TIMEOUT_SEC", 30),
	}
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		return n
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getEnvFloat64(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return n
}
