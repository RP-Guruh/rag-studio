package main

import (
	"embed"
	"log"

	"rag_project/internal/service"
	"rag_project/internal/settings"
	"rag_project/internal/training"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

// main is the composition root: it wires the persistence layer into the
// Wails-bindable services and starts the desktop application.
func main() {
	store, err := settings.OpenDefault()
	if err != nil {
		log.Fatalf("open settings database: %v", err)
	}
	defer store.Close()

	app := application.New(application.Options{
		Name:        "rag_project",
		Description: "Desktop RAG Studio for Knowledge Base and Persona Management",
		Services: []application.Service{
			application.NewService(service.NewGreetService()),
			application.NewService(service.NewSettingsService(store)),
			application.NewService(service.NewProjectService(store, training.NewManager(2))),
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
