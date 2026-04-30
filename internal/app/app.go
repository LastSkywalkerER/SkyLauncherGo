// Package app wires together the Wails application: window, services, and lifecycle.
package app

import (
	"embed"
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/config"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/hardware"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/logger"
)

const (
	AppName        = "SkyLauncher"
	AppDescription = "Cross-platform Minecraft launcher"
)

type Options struct {
	Version    string
	CommitHash string
	Assets     embed.FS
}

func Run(opts Options) error {
	log := logger.Default()
	log.Info("starting SkyLauncher", "version", opts.Version, "commit", opts.CommitHash)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	configSvc := config.NewService(cfg)
	hwSvc := hardware.NewService()

	app := application.New(application.Options{
		Name:        AppName,
		Description: AppDescription,
		Logger:      log,
		Services: []application.Service{
			application.NewService(configSvc),
			application.NewService(hwSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(opts.Assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           AppName,
		URL:             "/",
		Width:           1280,
		Height:          800,
		MinWidth:        960,
		MinHeight:       600,
		DevToolsEnabled: true,
		BackgroundColour: application.NewRGB(15, 15, 18),
	})

	return app.Run()
}
