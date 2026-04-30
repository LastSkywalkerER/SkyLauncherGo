// Package launcher is the Wails-bound facade that the frontend talks to
// in order to install and play modpack instances.
package launcher

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/auth"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/curseforge"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/installer"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/instances"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/modpack"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/xmcl/launch"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/xmcl/version"
)

// Service exposes high-level "install + play" verbs to the frontend.
type Service struct {
	Auth      *auth.Service
	Instances *instances.Manager
	Installer *installer.Service
}

func NewService(a *auth.Service, im *instances.Manager, is *installer.Service) *Service {
	return &Service{Auth: a, Instances: im, Installer: is}
}

// CreateAndInstallRequest is the payload used by the frontend to define a
// new instance — typically driven by a CurseForge modpack the user picked
// from search results.
type CreateAndInstallRequest struct {
	Name          string             `json:"name"`
	MCVersion     string             `json:"mcVersion"`
	Loader        installer.Modloader `json:"loader"`
	LoaderVersion string             `json:"loaderVersion"`
	ModpackModID  int                `json:"modpackModId,omitempty"`
	ModpackFileID int                `json:"modpackFileId,omitempty"`
	HeapMB        int                `json:"heapMB,omitempty"`
	IconURL       string             `json:"iconUrl,omitempty"`
}

// CreateAndInstall provisions a new instance directory and runs the full
// install pipeline. The returned Instance is ready for Launch.
func (s *Service) CreateAndInstall(ctx context.Context, req CreateAndInstallRequest) (*instances.Instance, error) {
	inst, err := s.Instances.Create(req.Name, req.MCVersion)
	if err != nil {
		return nil, err
	}
	plan := installer.Plan{
		InstanceDir:   s.Instances.Dir(inst.ID),
		MCVersion:     req.MCVersion,
		Loader:        req.Loader,
		LoaderVersion: req.LoaderVersion,
	}
	if req.ModpackFileID > 0 {
		plan.ModpackSource = &modpack.Source{
			Type:   "curseforge",
			ModID:  req.ModpackModID,
			FileID: req.ModpackFileID,
		}
	}
	res, err := s.Installer.Install(ctx, plan)
	if err != nil {
		// Best-effort cleanup so the user doesn't see a broken half-instance.
		_ = s.Instances.Delete(inst.ID, false)
		return nil, fmt.Errorf("install: %w", err)
	}
	inst.VersionID = res.VersionID
	inst.JavaPath = res.JavaPath
	inst.HeapMB = req.HeapMB
	inst.Loader = string(req.Loader)
	inst.LoaderVer = req.LoaderVersion
	inst.IconURL = req.IconURL
	if res.ManifestData != nil && res.ManifestData.Name != "" {
		inst.Modpack = res.ManifestData.Name
	}
	if err := s.Instances.Save(inst); err != nil {
		return nil, err
	}
	return inst, nil
}

// Launch starts the configured Minecraft instance using the active auth
// profile (or fails if none is set). The MC process is tracked by the
// instance manager so the UI can stop it later.
func (s *Service) Launch(ctx context.Context, instanceID string) error {
	prof := s.Auth.Current()
	if prof.Name == "" {
		return errors.New("no active profile; sign in first")
	}
	inst, err := s.Instances.Get(instanceID)
	if err != nil {
		return err
	}
	if inst.VersionID == "" || inst.JavaPath == "" {
		return errors.New("instance is not installed")
	}
	resolved, err := version.Parse(s.Instances.MinecraftDir(instanceID), inst.VersionID)
	if err != nil {
		return err
	}
	cmd, err := launch.Run(ctx, resolved, launch.Options{
		Folder:   s.Instances.MinecraftDir(instanceID),
		GameDir:  s.Instances.MinecraftDir(instanceID),
		JavaPath: inst.JavaPath,
		HeapMB:   inst.HeapMB,
		Profile: launch.Profile{
			Username:    prof.Name,
			UUID:        prof.ID,
			AccessToken: prof.AccessToken,
			UserType:    userTypeOf(prof.Type),
		},
		LauncherID: "skylauncher",
	})
	if err != nil {
		return err
	}
	if err := s.Instances.AttachProcess(instanceID, cmd); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	inst.LastPlayed = time.Now().UTC()
	_ = s.Instances.Save(inst)
	return nil
}

// Stop kills the MC process for the given instance, if it's running.
func (s *Service) Stop(ctx context.Context, instanceID string) error {
	return s.Instances.Stop(ctx, instanceID)
}

// List returns every instance descriptor (recently-played first).
func (s *Service) List() ([]instances.Instance, error) {
	return s.Instances.List()
}

// Running returns the IDs of currently-active instances.
func (s *Service) Running() []string {
	return s.Instances.RunningIDs()
}

// Search forwards to the CurseForge client (kept here so the frontend has
// one stable service surface).
func (s *Service) Search(ctx context.Context, cf *curseforge.Client, query string, page int) (*curseforge.SearchResponse, error) {
	return cf.SearchModpacks(ctx, curseforge.SearchParams{
		Search: query, Index: page * 25, PageSize: 25, SortField: 2, SortOrder: "desc",
	})
}

func userTypeOf(t string) string {
	switch t {
	case auth.TypeMicrosoft:
		return "msa"
	default:
		return "legacy"
	}
}
