package main

import (
	"context"
	"embed"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"rag_project/internal/auth"
	"rag_project/internal/projects"
	"rag_project/internal/service"
	"rag_project/internal/settings"
	"rag_project/internal/stack"
	"rag_project/internal/training"
	"rag_project/internal/update"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

// migrateOnStartup ensures new tables exist before services are registered.
func migrateOnStartup(store *settings.Store) {
	all, err := store.Get()
	if err != nil || !all.Database.Configured || !all.EmbeddingModel.Configured || all.EmbeddingModel.VectorDimension <= 0 {
		return
	}
	password, err := store.DatabasePassword()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := projects.MigratePostgres(ctx, all.Database, password, all.EmbeddingModel.VectorDimension); err != nil {
		slog.Warn("PostgreSQL startup migration failed", "error", err)
	}
}

// setupLogging mirrors structured logs to a file in the app config directory.
func setupLogging() {
	dir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	appDir := filepath.Join(dir, "rag-studio")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return
	}
	file, err := os.OpenFile(filepath.Join(appDir, "rag-studio.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	logger := slog.New(slog.NewTextHandler(io.MultiWriter(os.Stdout, file), nil))
	slog.SetDefault(logger)
}

// main is the composition root: it wires the persistence layer into the
// Wails-bindable services and starts the desktop application.
func main() {
	setupLogging()
	var store *settings.Store
	var err error
	if os.Getenv("RAG_STUDIO_SERVER") != "" {
		dir := os.Getenv("RAG_STUDIO_CONFIG_DIR")
		if dir == "" {
			base, err := os.UserConfigDir()
			if err != nil {
				log.Fatalf("locate config directory: %v", err)
			}
			dir = filepath.Join(base, "rag-studio")
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			log.Fatalf("create config directory: %v", err)
		}
		store, err = settings.Open(filepath.Join(dir, "settings.db"), settings.NewServerSecretStore(dir))
	} else {
		store, err = settings.OpenDefault()
	}
	if err != nil {
		log.Fatalf("open settings database: %v", err)
	}
	defer store.Close()

	authService := auth.NewService(store, store)
	migrateOnStartup(store)
	authService.PruneInvalidSessions()

	app := application.New(application.Options{
		Name:        "rag_project",
		Description: "Desktop RAG Studio for Knowledge Base and Persona Management",
		Services: []application.Service{
			application.NewService(service.NewGreetService()),
			application.NewService(service.NewSettingsService(store, authService)),
			application.NewService(update.NewService(authService)),
			application.NewService(stack.NewService(store, authService)),
			application.NewService(service.NewProjectService(store, training.NewManager(2), authService)),
			application.NewService(authService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "RAG Studio",
		Width:            1024,
		Height:           768,
		BackgroundColour: application.NewRGB(248, 250, 252),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
