// Package manifest fetches and caches Mojang's version_manifest_v2.json,
// the index of every official Minecraft version JSON.
package manifest

import (
	"context"
	"sync"
	"time"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/httpclient"
)

const URL = "https://launchermeta.mojang.com/mc/game/version_manifest_v2.json"

// Latest names the latest release/snapshot ids.
type Latest struct {
	Release  string `json:"release"`
	Snapshot string `json:"snapshot"`
}

// VersionEntry references one Minecraft version JSON.
type VersionEntry struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"` // "release" | "snapshot" | "old_beta" | "old_alpha"
	URL             string    `json:"url"`
	Time            time.Time `json:"time"`
	ReleaseTime     time.Time `json:"releaseTime"`
	SHA1            string    `json:"sha1"`
	ComplianceLevel int       `json:"complianceLevel"`
}

// Manifest is the parsed manifest_v2 root document.
type Manifest struct {
	Latest   Latest         `json:"latest"`
	Versions []VersionEntry `json:"versions"`
}

// Fetcher pulls the manifest with in-memory caching.
type Fetcher struct {
	client *httpclient.Client

	mu       sync.Mutex
	cached   *Manifest
	cachedAt time.Time
	ttl      time.Duration
}

func NewFetcher(c *httpclient.Client) *Fetcher {
	return &Fetcher{client: c, ttl: 10 * time.Minute}
}

func (f *Fetcher) Get(ctx context.Context) (*Manifest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cached != nil && time.Since(f.cachedAt) < f.ttl {
		return f.cached, nil
	}
	var m Manifest
	if err := f.client.GetJSON(ctx, URL, nil, &m); err != nil {
		return nil, err
	}
	f.cached = &m
	f.cachedAt = time.Now()
	return f.cached, nil
}

// Find returns the entry for the given id, or nil.
func (m *Manifest) Find(id string) *VersionEntry {
	for i := range m.Versions {
		if m.Versions[i].ID == id {
			return &m.Versions[i]
		}
	}
	return nil
}
