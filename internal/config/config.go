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
}

func LoadFromEnv() Config {
	return Config{
		ListenAddr:       getEnv("LISTEN_ADDR", ":8080"),
		LexiconDir:       getEnv("LEXICON_DIR", "./Vocabulary"),
		ReplaceSymbol:    getEnv("REPLACE_SYMBOL", "*"),
		EnableBoundary:   getEnvBool("ENABLE_BOUNDARY", true),
		APIKey:           os.Getenv("API_KEY"),
		BaseRPS:          getEnvInt("BASE_RPS", 600),
		MaxBodyBytes:     getEnvInt64("MAX_BODY_BYTES", 1<<20),
		AsyncQueueLength: getEnvInt("ASYNC_QUEUE_LENGTH", 128),
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
