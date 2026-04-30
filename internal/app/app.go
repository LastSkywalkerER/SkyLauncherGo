// Package app wires together the Wails application: window, services, and lifecycle.
package app

import (
	"embed"
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/auth"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/config"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/curseforge"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/hardware"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/httpclient"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/installer"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/instances"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/java"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/launcher"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/logger"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/paths"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/progress"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/updater"
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

// Run constructs every service, wires them into the Wails application, and
// blocks until the user closes the window.
func Run(opts Options) error {
	log := logger.Default()
	log.Info("starting SkyLauncher", "version", opts.Version, "commit", opts.CommitHash)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	instanceRoot, err := paths.InstancesDir()
	if err != nil {
		return err
	}
	bus := progress.NewBus()
	httpC := httpclient.New()
	cfClient := curseforge.NewClient(httpC)
	javaProv := java.NewProvider(httpC, bus)
	instMgr, err := instances.NewManager(instanceRoot)
	if err != nil {
		return err
	}
	installSvc := installer.NewService(httpC, cfClient, javaProv, bus)
	authSvc := auth.NewService(httpC)
	if cfg.User.Name != "" {
		// Restore the previously-active offline profile (no MS refresh in MVP).
		if cfg.User.Type == auth.TypeOffline {
			_, _ = authSvc.LoginOffline(cfg.User.Name)
		}
	}
	configSvc := config.NewService(cfg)
	launchSvc := launcher.NewService(authSvc, instMgr, installSvc)
	hwSvc := hardware.NewService()
	updaterSvc := updater.NewService(httpC, bus, opts.Version)

	// progressBridge is a tiny progress.Sink that re-emits events on the
	// Wails bus so the frontend can subscribe to "process-progress".
	br := &progressBridge{}
	bus.Subscribe(br)

	app := application.New(application.Options{
		Name:        AppName,
		Description: AppDescription,
		Logger:      log,
		Services: []application.Service{
			application.NewService(configSvc),
			application.NewService(hwSvc),
			application.NewService(authSvc),
			application.NewService(cfClient),
			application.NewService(launchSvc),
			application.NewService(installSvc),
			application.NewService(updaterSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(opts.Assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	br.app = app

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            AppName,
		URL:              "/",
		Width:            1280,
		Height:           800,
		MinWidth:         960,
		MinHeight:        600,
		DevToolsEnabled:  true,
		BackgroundColour: application.NewRGB(15, 15, 18),
	})

	return app.Run()
}

// progressBridge forwards progress events to the Wails frontend via
// EmitEvent under the legacy event name "process-progress" so the
// existing React listeners keep working.
type progressBridge struct {
	app *application.App
}

func (p *progressBridge) Emit(e progress.Event) {
	if p.app == nil {
		return
	}
	p.app.Event.Emit("process-progress", e)
}
