// Package fabric installs the Fabric loader for a vanilla Minecraft version.
//
// Fabric Meta returns a fully-formed Minecraft version profile JSON
// (with `inheritsFrom` pointing at vanilla); we just write it to disk and
// let the standard installer pull libraries via the parent chain.
package fabric

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/httpclient"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/xmcl/version"
)

const metaBase = "https://meta.fabricmc.net/v2"

// LoaderEntry is one entry in the loader version list for a given MC version.
type LoaderEntry struct {
	Loader struct {
		Separator string `json:"separator"`
		Build     int    `json:"build"`
		Maven     string `json:"maven"`
		Version   string `json:"version"`
		Stable    bool   `json:"stable"`
	} `json:"loader"`
	Intermediary struct {
		Maven   string `json:"maven"`
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
	} `json:"intermediary"`
	LauncherMeta any `json:"launcherMeta"`
}

// ListLoaders fetches all Fabric loader versions compatible with mcVersion,
// newest first.
func ListLoaders(ctx context.Context, c *httpclient.Client, mcVersion string) ([]LoaderEntry, error) {
	var out []LoaderEntry
	url := fmt.Sprintf("%s/versions/loader/%s", metaBase, mcVersion)
	if err := c.GetJSON(ctx, url, nil, &out); err != nil {
		return nil, fmt.Errorf("fabric loaders: %w", err)
	}
	return out, nil
}

// LatestStableLoader picks the newest stable loader for mcVersion.
func LatestStableLoader(ctx context.Context, c *httpclient.Client, mcVersion string) (string, error) {
	loaders, err := ListLoaders(ctx, c, mcVersion)
	if err != nil {
		return "", err
	}
	for _, l := range loaders {
		if l.Loader.Stable {
			return l.Loader.Version, nil
		}
	}
	if len(loaders) > 0 {
		return loaders[0].Loader.Version, nil
	}
	return "", fmt.Errorf("no fabric loader available for %s", mcVersion)
}

// ProfileID is the canonical version id Fabric uses on disk.
func ProfileID(mc, loader string) string {
	return fmt.Sprintf("fabric-loader-%s-%s", loader, mc)
}

// Install fetches the Fabric profile JSON and writes it to versions/<id>/<id>.json.
// Caller should then run installer.InstallVersion (etc.) to pull libraries.
func Install(ctx context.Context, c *httpclient.Client, folder, mcVersion, loaderVersion string) (string, error) {
	id := ProfileID(mcVersion, loaderVersion)
	url := fmt.Sprintf("%s/versions/loader/%s/%s/profile/json", metaBase, mcVersion, loaderVersion)

	var raw json.RawMessage
	if err := c.GetJSON(ctx, url, nil, &raw); err != nil {
		return "", fmt.Errorf("fetch fabric profile: %w", err)
	}
	dest := version.VersionJSONPath(folder, id)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, raw, 0o644); err != nil {
		return "", err
	}
	return id, nil
}
