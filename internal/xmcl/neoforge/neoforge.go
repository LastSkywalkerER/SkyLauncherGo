// Package neoforge installs the NeoForge modloader (Forge fork, MC 1.20.2+).
//
// NeoForge ships a Forge-style installer JAR; we delegate to it the same
// way we do for Forge.
package neoforge

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/httpclient"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/progress"
)

const (
	mavenMetadataURL = "https://maven.neoforged.net/releases/net/neoforged/neoforge/maven-metadata.xml"
	installerTpl     = "https://maven.neoforged.net/releases/net/neoforged/neoforge/%s/neoforge-%s-installer.jar"
)

type mavenMetadata struct {
	XMLName    xml.Name `xml:"metadata"`
	Versioning struct {
		Versions struct {
			Version []string `xml:"version"`
		} `xml:"versions"`
	} `xml:"versioning"`
}

// ListVersions returns every published NeoForge version, newest first.
//
// NeoForge versions look like "20.4.237" where the leading "20.4" mirrors
// the MC version (1.20.4). Callers should filter by prefix.
func ListVersions(ctx context.Context, c *httpclient.Client) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mavenMetadataURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("neoforge metadata: %s", resp.Status)
	}
	var m mavenMetadata
	if err := xml.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	out := append([]string(nil), m.Versioning.Versions.Version...)
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out, nil
}

// VersionsForMC filters NeoForge releases that match the given MC version
// (1.20.2 → "20.2", 1.21 → "21.0", etc.).
func VersionsForMC(all []string, mc string) []string {
	prefix := mcPrefix(mc)
	var out []string
	for _, v := range all {
		if strings.HasPrefix(v, prefix+".") || v == prefix {
			out = append(out, v)
		}
	}
	return out
}

// mcPrefix maps "1.20.2" → "20.2", "1.21" → "21.0".
func mcPrefix(mc string) string {
	parts := strings.SplitN(mc, ".", 3)
	if len(parts) < 2 {
		return mc
	}
	major := parts[1]
	patch := "0"
	if len(parts) >= 3 {
		patch = parts[2]
	}
	return major + "." + patch
}

// Install downloads and runs the NeoForge installer JAR.
func Install(
	ctx context.Context,
	c *httpclient.Client,
	rep progress.Reporter,
	folder, javaPath, neoVersion string,
) (string, error) {
	if javaPath == "" {
		return "", fmt.Errorf("neoforge install: javaPath required")
	}
	if rep == nil {
		rep = progress.Noop{}
	}
	tmp, err := os.MkdirTemp("", "skylauncher-neoforge-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	url := fmt.Sprintf(installerTpl, neoVersion, neoVersion)
	jar := filepath.Join(tmp, "neoforge-installer.jar")
	if err := c.Download(ctx, httpclient.DownloadOptions{
		URL: url, DestPath: jar,
		Reporter: rep, TaskID: neoVersion, Stage: progress.StageInstall,
	}); err != nil {
		return "", fmt.Errorf("download neoforge installer: %w", err)
	}

	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", err
	}
	profilesPath := filepath.Join(folder, "launcher_profiles.json")
	if _, err := os.Stat(profilesPath); os.IsNotExist(err) {
		_ = os.WriteFile(profilesPath, []byte(`{"profiles":{},"selectedProfile":""}`), 0o644)
	}

	cmd := exec.CommandContext(ctx, javaPath, "-jar", jar, "--installClient", folder)
	cmd.Dir = folder
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("neoforge installer: %w", err)
	}
	return ProfileID(neoVersion), nil
}

// ProfileID is the version id NeoForge writes under versions/.
func ProfileID(neoVersion string) string {
	return "neoforge-" + neoVersion
}
