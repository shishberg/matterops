package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/shishberg/matterops/internal/app"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	envPath := flag.String("env", ".env", "path to .env file")
	flag.Parse()

	a, err := app.New(*configPath, *envPath)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		a.Shutdown()
		cancel()
	}()

	if err := a.Run(ctx); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
