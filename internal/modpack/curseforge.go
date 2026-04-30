// Package modpack installs a Minecraft modpack into an isolated instance
// directory. The CurseForge ZIP format is the only one supported in MVP;
// the package is structured so a Modrinth (.mrpack) provider can be added
// later behind the same Provider interface.
package modpack

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/curseforge"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/httpclient"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/progress"
)

// Source describes where a modpack archive comes from.
type Source struct {
	Type   string // "curseforge" | "local"
	ModID  int    // for "curseforge"
	FileID int    // for "curseforge"
	Path   string // for "local" — absolute path to .zip
}

// Result captures the modpack manifest extracted from the archive plus
// the path to where overrides/ was unpacked.
type Result struct {
	Name        string
	Version     string
	Author      string
	Manifest    *curseforge.ModpackManifest
	InstanceDir string
}

// Provider is the abstract interface fulfilled by curseforge / modrinth /
// local providers. Modrinth not yet implemented.
type Provider interface {
	Install(ctx context.Context, src Source, instanceDir string) (*Result, error)
}

// CurseForgeProvider fetches a CF modpack zip, extracts the manifest, and
// stages overrides/. It does NOT install the modloader or vanilla — that's
// the InstallerService's job.
type CurseForgeProvider struct {
	HTTP     *httpclient.Client
	CF       *curseforge.Client
	Reporter progress.Reporter
}

func NewCurseForgeProvider(h *httpclient.Client, cf *curseforge.Client, rep progress.Reporter) *CurseForgeProvider {
	if rep == nil {
		rep = progress.Noop{}
	}
	return &CurseForgeProvider{HTTP: h, CF: cf, Reporter: rep}
}

func (p *CurseForgeProvider) Install(ctx context.Context, src Source, instanceDir string) (*Result, error) {
	if src.Type != "curseforge" {
		return nil, fmt.Errorf("unsupported source %q", src.Type)
	}
	if src.FileID == 0 {
		return nil, errors.New("curseforge source: fileID required")
	}
	file, err := p.CF.GetFile(ctx, src.ModID, src.FileID)
	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}
	url := curseforge.ResolveDownloadURL(*file)
	if url == "" {
		return nil, fmt.Errorf("file %d has no resolvable downloadUrl", file.ID)
	}

	tmp, err := os.MkdirTemp("", "skylauncher-modpack-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	zipPath := filepath.Join(tmp, file.FileName)
	if err := p.HTTP.Download(ctx, httpclient.DownloadOptions{
		URL: url, DestPath: zipPath, Size: file.FileLength,
		SHA1: extractHash(file.Hashes, 1), Headers: map[string]string{"Referer": "https://www.curseforge.com/"},
		Reporter: p.Reporter, TaskID: file.FileName, Stage: progress.StageDownload,
	}); err != nil {
		return nil, fmt.Errorf("download modpack zip: %w", err)
	}

	manifest, err := readManifest(zipPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest.json: %w", err)
	}

	dotMC := filepath.Join(instanceDir, ".minecraft")
	if err := os.MkdirAll(dotMC, 0o755); err != nil {
		return nil, err
	}

	overridesRoot := manifest.Overrides
	if overridesRoot == "" {
		overridesRoot = "overrides"
	}
	if err := extractOverrides(zipPath, overridesRoot, dotMC); err != nil {
		return nil, fmt.Errorf("extract overrides: %w", err)
	}

	return &Result{
		Name:        manifest.Name,
		Version:     manifest.Version,
		Author:      manifest.Author,
		Manifest:    manifest,
		InstanceDir: instanceDir,
	}, nil
}

// readManifest pulls manifest.json out of the archive without unpacking.
func readManifest(zipPath string) (*curseforge.ModpackManifest, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == "manifest.json" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, err
			}
			var m curseforge.ModpackManifest
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, err
			}
			return &m, nil
		}
	}
	return nil, errors.New("manifest.json not found in archive")
}

// extractOverrides copies entries under overrides/ from src zip into destDir.
func extractOverrides(zipPath, overridesRoot, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	prefix := overridesRoot + "/"
	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, prefix) {
			continue
		}
		rel := strings.TrimPrefix(f.Name, prefix)
		if rel == "" {
			continue
		}
		out := filepath.Join(destDir, filepath.FromSlash(rel))
		if !strings.HasPrefix(filepath.Clean(out)+string(os.PathSeparator), filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(out, 0o755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.Create(out)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(w, rc); err != nil {
			rc.Close()
			w.Close()
			return err
		}
		rc.Close()
		w.Close()
	}
	return nil
}

func extractHash(hashes []curseforge.Hash, algo int) string {
	for _, h := range hashes {
		if h.Algo == algo {
			return h.Value
		}
	}
	return ""
}
