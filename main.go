package main

import (
	"embed"
	"log"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

var (
	Version    = "0.0.0-dev"
	CommitHash = "unknown"
)

func main() {
	if err := app.Run(app.Options{
		Version:    Version,
		CommitHash: CommitHash,
		Assets:     assets,
	}); err != nil {
		log.Fatalf("application terminated: %v", err)
	}
}
