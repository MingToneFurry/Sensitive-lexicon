package main

import (
	"flag"
	"log"

	"github.com/MingToneFurry/Sensitive-lexicon/internal/config"
	"github.com/MingToneFurry/Sensitive-lexicon/internal/server"
)

func main() {
	configFile := flag.String("config", "config.json", "path to JSON config file (default: config.json; file is optional — falls back to env vars)")
	flag.Parse()

	cfg := config.Load(*configFile)
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("create server: %v", err)
	}
	log.Printf("sensitive server listening on %s", cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
