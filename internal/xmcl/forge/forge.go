// Package forge installs the Forge modloader by running the official
// installer JAR in headless --installClient mode.
//
// Forge's classic installer hasn't been a single-step download for years;
// it's a small JAR that:
//   1. fetches additional libraries (some are processed/repackaged on disk),
//   2. produces the patched <mc>-forge-<ver> profile under versions/, and
//   3. populates the libraries/ tree.
//
// Re-implementing the processor pipeline in Go is brittle (each MC era ships
// a different format), so we delegate to the JAR. Java is required and
// must already be installed (use internal/java to provision it).
package forge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/httpclient"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/progress"
)

const (
	promotionsURL = "https://files.minecraftforge.net/net/minecraftforge/forge/promotions_slim.json"
	installerTpl  = "https://maven.minecraftforge.net/net/minecraftforge/forge/%s-%s/forge-%s-%s-installer.jar"
)

// Promotions holds Forge's recommended/latest version map per MC version.
type Promotions struct {
	Promos map[string]string `json:"promos"` // e.g. "1.20.1-recommended" → "47.2.0"
}

// FetchPromotions returns the recommended/latest Forge versions index.
func FetchPromotions(ctx context.Context, c *httpclient.Client) (*Promotions, error) {
	var p Promotions
	if err := c.GetJSON(ctx, promotionsURL, nil, &p); err != nil {
		return nil, fmt.Errorf("forge promotions: %w", err)
	}
	return &p, nil
}

// LatestRecommended returns the recommended Forge version for an MC version,
// falling back to "latest" if no recommended build exists.
func (p *Promotions) LatestRecommended(mc string) string {
	if v, ok := p.Promos[mc+"-recommended"]; ok {
		return v
	}
	if v, ok := p.Promos[mc+"-latest"]; ok {
		return v
	}
	return ""
}

// AvailableForMC enumerates every key in promos that targets mc.
func (p *Promotions) AvailableForMC(mc string) []string {
	var out []string
	for k, v := range p.Promos {
		if len(k) > len(mc) && k[:len(mc)] == mc && k[len(mc)] == '-' {
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// Install downloads the Forge installer JAR and runs it with --installClient
// pointed at folder.
func Install(
	ctx context.Context,
	c *httpclient.Client,
	rep progress.Reporter,
	folder, javaPath, mc, forgeVersion string,
) (string, error) {
	if javaPath == "" {
		return "", fmt.Errorf("forge install: javaPath required")
	}
	if rep == nil {
		rep = progress.Noop{}
	}
	tmp, err := os.MkdirTemp("", "skylauncher-forge-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	url := fmt.Sprintf(installerTpl, mc, forgeVersion, mc, forgeVersion)
	jar := filepath.Join(tmp, "forge-installer.jar")
	if err := c.Download(ctx, httpclient.DownloadOptions{
		URL: url, DestPath: jar,
		Reporter: rep, TaskID: forgeVersion, Stage: progress.StageInstall,
	}); err != nil {
		return "", fmt.Errorf("download forge installer: %w", err)
	}

	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", err
	}
	// Forge's installer needs a launcher_profiles.json to claim the install
	// completed; create an empty stub if missing.
	profilesPath := filepath.Join(folder, "launcher_profiles.json")
	if _, err := os.Stat(profilesPath); os.IsNotExist(err) {
		_ = os.WriteFile(profilesPath, []byte(`{"profiles":{},"selectedProfile":""}`), 0o644)
	}

	cmd := exec.CommandContext(ctx, javaPath, "-jar", jar, "--installClient", folder)
	cmd.Dir = folder
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("forge installer: %w", err)
	}
	return ProfileID(mc, forgeVersion), nil
}

// ProfileID is the version id Forge writes under versions/.
func ProfileID(mc, forgeVersion string) string {
	return fmt.Sprintf("%s-forge-%s", mc, forgeVersion)
}
