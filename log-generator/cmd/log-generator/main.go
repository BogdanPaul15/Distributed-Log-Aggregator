package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"log-generator/internal/api"
	"log-generator/internal/config"
	"log-generator/internal/engine"
	"log-generator/internal/generator/random"
	"log-generator/internal/storage"
	"log-generator/internal/storage/console"
	"log-generator/internal/storage/http"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Loaded Config -> Workers: %d, DefaultRate: %d", cfg.Engine.Workers, cfg.Engine.DefaultRate)

	var store storage.Storage

	switch cfg.Storage.Type {
	case config.StorageConsole:
		store = console.NewConsoleStorage()
	case config.StorageHTTP:
		store = http.NewHTTPStorage(cfg.Storage.HTTP)
	default:
		log.Fatalf("Unknown storage type: %s", cfg.Storage.Type)
	}

	generator := random.NewRandomGenerator(cfg.Generator)

	eng := engine.NewEngine(generator, store, cfg.Engine)

	server := api.NewServer(eng, generator)

	go func() {
		log.Println("Control API listening on :8081")
		if err := server.ListenAndServe(":8081"); err != nil {
			log.Printf("API Error: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\nReceived shutdown signal...")
		cancel()
	}()

	log.Printf("Log Generator started with %d workers. Press Ctrl+C to stop.", cfg.Engine.Workers)
	eng.Start(ctx)
}
