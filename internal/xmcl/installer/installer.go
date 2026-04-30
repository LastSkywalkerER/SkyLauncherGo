// Package installer downloads everything required to launch a specific
// Minecraft version: the version JSON, client jar, libraries, asset index
// and assets.
//
// It mirrors @xmcl/installer's installVersionTask / installLibrariesTask /
// installAssetsTask, simplified for the launcher's actual usage pattern.
package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/httpclient"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/progress"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/xmcl/task"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/xmcl/version"
)

const (
	resourcesBase = "https://resources.download.minecraft.net"
)

// Installer wires together the HTTP client and progress reporter shared by
// every install step.
type Installer struct {
	HTTP     *httpclient.Client
	Reporter progress.Reporter
}

func New(client *httpclient.Client, rep progress.Reporter) *Installer {
	if rep == nil {
		rep = progress.Noop{}
	}
	return &Installer{HTTP: client, Reporter: rep}
}

// InstallVersion downloads the version JSON to versions/<id>/<id>.json plus
// the client jar. Caller passes the manifest entry from version_manifest_v2.
func (in *Installer) InstallVersion(ctx context.Context, t *task.Task, folder, id, jsonURL, jsonSHA1 string) error {
	jsonPath := version.VersionJSONPath(folder, id)
	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
		return err
	}
	if err := in.HTTP.Download(ctx, httpclient.DownloadOptions{
		URL: jsonURL, DestPath: jsonPath, SHA1: jsonSHA1,
		Reporter: in.Reporter, TaskID: id, Stage: progress.StageManifest,
	}); err != nil {
		return fmt.Errorf("download version json: %w", err)
	}

	resolved, err := version.Parse(folder, id)
	if err != nil {
		return err
	}
	jarPath := version.VersionJARPath(folder, id)
	if resolved.ClientDownload.URL != "" {
		if err := in.HTTP.Download(ctx, httpclient.DownloadOptions{
			URL: resolved.ClientDownload.URL, DestPath: jarPath,
			SHA1: resolved.ClientDownload.SHA1, Size: resolved.ClientDownload.Size,
			Reporter: in.Reporter, TaskID: id, Stage: progress.StageDownload,
		}); err != nil {
			return fmt.Errorf("download client jar: %w", err)
		}
	}
	if t != nil {
		t.Add(1)
	}
	return nil
}

// InstallLibraries downloads (and where appropriate, extracts) every library
// declared in the resolved version, honouring per-OS rules and natives.
func (in *Installer) InstallLibraries(ctx context.Context, t *task.Task, folder string, resolved *version.Resolved) error {
	libsDir := filepath.Join(folder, "libraries")
	nativesDir := filepath.Join(folder, "versions", resolved.ID, "natives")
	if err := os.MkdirAll(nativesDir, 0o755); err != nil {
		return err
	}

	plan := planLibraries(libsDir, resolved.Libraries)
	if t != nil {
		t.SetTotal(int64(len(plan)))
	}

	fns := make([]task.Func, 0, len(plan))
	for _, item := range plan {
		item := item
		fns = append(fns, func(ctx context.Context, sub *task.Task) error {
			if err := in.HTTP.Download(ctx, httpclient.DownloadOptions{
				URL: item.URL, DestPath: item.Dest,
				SHA1: item.SHA1, Size: item.Size,
				Reporter: in.Reporter, TaskID: resolved.ID, Stage: progress.StageLibraries,
			}); err != nil {
				return fmt.Errorf("library %s: %w", item.URL, err)
			}
			if item.IsNative {
				if err := extractZipExcluding(item.Dest, nativesDir, item.ExtractExclude); err != nil {
					return fmt.Errorf("extract native %s: %w", filepath.Base(item.Dest), err)
				}
			}
			if t != nil {
				t.Add(1)
			}
			return nil
		})
	}
	root := t
	if root == nil {
		root = task.New("install-libraries", nil)
	}
	return root.Parallel(ctx, 8, fns)
}

// InstallAssets fetches the asset index and every individual asset object.
func (in *Installer) InstallAssets(ctx context.Context, t *task.Task, folder string, resolved *version.Resolved) error {
	if resolved.AssetIndex.URL == "" {
		return errors.New("no asset index in version")
	}
	indexDir := filepath.Join(folder, "assets", "indexes")
	if err := os.MkdirAll(indexDir, 0o755); err != nil {
		return err
	}
	indexPath := filepath.Join(indexDir, resolved.AssetIndex.ID+".json")
	if err := in.HTTP.Download(ctx, httpclient.DownloadOptions{
		URL: resolved.AssetIndex.URL, DestPath: indexPath,
		SHA1: resolved.AssetIndex.SHA1, Size: resolved.AssetIndex.Size,
		Reporter: in.Reporter, TaskID: resolved.ID, Stage: progress.StageAssets,
	}); err != nil {
		return err
	}

	indexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}
	var idx assetIndex
	if err := json.Unmarshal(indexBytes, &idx); err != nil {
		return err
	}
	objectsDir := filepath.Join(folder, "assets", "objects")

	type entry struct {
		hash string
		size int64
	}
	all := make([]entry, 0, len(idx.Objects))
	for _, o := range idx.Objects {
		all = append(all, entry{o.Hash, o.Size})
	}
	if t != nil {
		t.SetTotal(int64(len(all)))
	}

	fns := make([]task.Func, 0, len(all))
	for _, e := range all {
		e := e
		first := e.hash[:2]
		dest := filepath.Join(objectsDir, first, e.hash)
		fns = append(fns, func(ctx context.Context, sub *task.Task) error {
			if existing, err := httpclient.ChecksumFile(dest); err == nil {
				// Asset hashes are SHA-1, but if SHA-256 happens to match
				// (it never will here) we'd skip. Fall back to size check.
				if st, statErr := os.Stat(dest); statErr == nil && st.Size() == e.size && existing != "" {
					if t != nil {
						t.Add(1)
					}
					return nil
				}
			}
			url := fmt.Sprintf("%s/%s/%s", resourcesBase, first, e.hash)
			if err := in.HTTP.Download(ctx, httpclient.DownloadOptions{
				URL: url, DestPath: dest, SHA1: e.hash, Size: e.size,
				Reporter: in.Reporter, TaskID: resolved.ID, Stage: progress.StageAssets,
			}); err != nil {
				return err
			}
			if t != nil {
				t.Add(1)
			}
			return nil
		})
	}
	root := t
	if root == nil {
		root = task.New("install-assets", nil)
	}
	return root.Parallel(ctx, 16, fns)
}

// Install is the convenience wrapper that runs the full vanilla pipeline:
// version json → libraries → assets.
func (in *Installer) Install(ctx context.Context, folder, id, jsonURL, jsonSHA1 string) error {
	root := task.New("install-"+id, nil)
	if err := in.InstallVersion(ctx, root, folder, id, jsonURL, jsonSHA1); err != nil {
		return err
	}
	resolved, err := version.Parse(folder, id)
	if err != nil {
		return err
	}
	if err := in.InstallLibraries(ctx, root.Sub("libraries"), folder, resolved); err != nil {
		return err
	}
	if err := in.InstallAssets(ctx, root.Sub("assets"), folder, resolved); err != nil {
		return err
	}
	return nil
}

type assetIndex struct {
	Objects map[string]struct {
		Hash string `json:"hash"`
		Size int64  `json:"size"`
	} `json:"objects"`
}

type libraryItem struct {
	URL            string
	Dest           string
	SHA1           string
	Size           int64
	IsNative       bool
	ExtractExclude []string
}

func planLibraries(libsDir string, libs []version.Library) []libraryItem {
	var out []libraryItem
	for _, l := range libs {
		if !version.MatchRules(l.Rules, nil) {
			continue
		}
		// Modern artifact
		if l.Downloads != nil && l.Downloads.Artifact != nil {
			a := l.Downloads.Artifact
			if a.URL != "" {
				out = append(out, libraryItem{
					URL: a.URL, Dest: filepath.Join(libsDir, filepath.FromSlash(a.Path)),
					SHA1: a.SHA1, Size: a.Size,
				})
			}
		}
		// Native classifier (legacy + modern)
		nativeKey := version.LibraryNativeClassifier(l)
		if nativeKey != "" && l.Downloads != nil {
			if a := l.Downloads.Classifiers[nativeKey]; a != nil && a.URL != "" {
				exclude := []string{}
				if l.Extract != nil {
					exclude = l.Extract.Exclude
				}
				out = append(out, libraryItem{
					URL: a.URL, Dest: filepath.Join(libsDir, filepath.FromSlash(a.Path)),
					SHA1: a.SHA1, Size: a.Size,
					IsNative: true, ExtractExclude: exclude,
				})
			}
		}
		// Maven-only entries (e.g. Forge/Fabric libs that only carry name+url)
		if l.Downloads == nil && l.Name != "" {
			path := version.MavenPath(l.Name)
			base := strings.TrimRight(l.URL, "/")
			if base == "" {
				base = "https://libraries.minecraft.net"
			}
			out = append(out, libraryItem{
				URL: base + "/" + path,
				Dest: filepath.Join(libsDir, filepath.FromSlash(path)),
			})
		}
	}
	return out
}

// CheckURL returns true if a HEAD on the URL succeeds (used in tests).
func CheckURL(ctx context.Context, c *httpclient.Client, url string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode < 400, nil
}
