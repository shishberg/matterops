package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/shishberg/matterops/internal/app"
)

//go:embed templates/index.html
var templatesEmbedFS embed.FS

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	envPath := flag.String("env", ".env", "path to .env file")
	flag.Parse()

	// Sub into the "templates" directory so dashboard sees index.html at root.
	templatesFS, err := fs.Sub(templatesEmbedFS, "templates")
	if err != nil {
		log.Fatalf("Failed to load embedded templates: %v", err)
	}

	a, err := app.New(*configPath, *envPath, templatesFS)
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
