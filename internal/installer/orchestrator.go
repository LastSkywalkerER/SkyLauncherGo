// Package installer ties the per-domain installers (vanilla, fabric, forge,
// neoforge, modpack, curseforge) together into a single orchestrated
// pipeline that produces a ready-to-launch instance.
package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/curseforge"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/httpclient"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/java"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/modpack"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/progress"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/xmcl/fabric"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/xmcl/forge"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/xmcl/installer"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/xmcl/manifest"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/xmcl/neoforge"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/xmcl/version"
)

// Modloader is the kind of loader to install on top of vanilla.
type Modloader string

const (
	LoaderVanilla  Modloader = "vanilla"
	LoaderFabric   Modloader = "fabric"
	LoaderForge    Modloader = "forge"
	LoaderNeoForge Modloader = "neoforge"
)

// Plan is the input to the orchestrator: one fully-described install job.
type Plan struct {
	InstanceDir   string             // .minecraft root for this instance
	MCVersion     string             // e.g. "1.20.1"
	Loader        Modloader          // vanilla|fabric|forge|neoforge
	LoaderVersion string             // empty → resolve recommended/stable
	ModpackSource *modpack.Source    // optional CurseForge zip to apply
}

// Result describes what the orchestrator produced.
type Result struct {
	VersionID    string                       // resolved version id (e.g. "1.20.1-fabric-...")
	JavaPath     string                       // path to java(.exe) provisioned for this version
	ModpackName  string                       // empty if Plan.ModpackSource was nil
	ManifestData *curseforge.ModpackManifest  // when modpack provided
}

// Service is the top-level installer used by the Wails service layer.
type Service struct {
	HTTP     *httpclient.Client
	CF       *curseforge.Client
	Java     *java.Provider
	Reporter progress.Reporter
}

func NewService(h *httpclient.Client, cf *curseforge.Client, jp *java.Provider, rep progress.Reporter) *Service {
	if rep == nil {
		rep = progress.Noop{}
	}
	return &Service{HTTP: h, CF: cf, Java: jp, Reporter: rep}
}

// Install runs the full pipeline for one instance. Steps are sequential
// because each depends on the previous (manifest → modloader → libraries).
func (s *Service) Install(ctx context.Context, plan Plan) (*Result, error) {
	if plan.MCVersion == "" {
		return nil, errors.New("install plan: MCVersion required")
	}
	if plan.InstanceDir == "" {
		return nil, errors.New("install plan: InstanceDir required")
	}
	res := &Result{}

	// 1. Optionally stage modpack overrides + override loader fields from manifest.
	if plan.ModpackSource != nil {
		prov := modpack.NewCurseForgeProvider(s.HTTP, s.CF, s.Reporter)
		mp, err := prov.Install(ctx, *plan.ModpackSource, plan.InstanceDir)
		if err != nil {
			return nil, fmt.Errorf("modpack: %w", err)
		}
		res.ModpackName = mp.Name
		res.ManifestData = mp.Manifest
		if mp.Manifest.Minecraft.Version != "" {
			plan.MCVersion = mp.Manifest.Minecraft.Version
		}
		if loader, lver, ok := primaryLoader(mp.Manifest.Minecraft.ModLoaders); ok {
			plan.Loader = loader
			plan.LoaderVersion = lver
		}
	}

	// 2. Install vanilla.
	dotMC := filepath.Join(plan.InstanceDir, ".minecraft")
	mfFetcher := manifest.NewFetcher(s.HTTP)
	manif, err := mfFetcher.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	vanilla := manif.Find(plan.MCVersion)
	if vanilla == nil {
		return nil, fmt.Errorf("unknown MC version %s", plan.MCVersion)
	}
	xmclInst := installer.New(s.HTTP, s.Reporter)
	if err := xmclInst.Install(ctx, dotMC, vanilla.ID, vanilla.URL, vanilla.SHA1); err != nil {
		return nil, fmt.Errorf("install vanilla: %w", err)
	}
	resolvedID := vanilla.ID

	// 3. Provision Java for this MC era. Used by both forge installer and launcher.
	resolved, err := version.Parse(dotMC, vanilla.ID)
	if err != nil {
		return nil, err
	}
	javaPath, err := s.Java.EnsureJRE(ctx, java.MajorFor(resolved))
	if err != nil {
		return nil, fmt.Errorf("ensure jre: %w", err)
	}
	res.JavaPath = javaPath

	// 4. Layer modloader, if requested.
	switch plan.Loader {
	case "", LoaderVanilla:
		// nothing further
	case LoaderFabric:
		ver := plan.LoaderVersion
		if ver == "" {
			ver, err = fabric.LatestStableLoader(ctx, s.HTTP, plan.MCVersion)
			if err != nil {
				return nil, err
			}
		}
		id, err := fabric.Install(ctx, s.HTTP, dotMC, plan.MCVersion, ver)
		if err != nil {
			return nil, fmt.Errorf("install fabric: %w", err)
		}
		// Pull libraries declared by the fabric profile.
		fr, err := version.Parse(dotMC, id)
		if err != nil {
			return nil, err
		}
		if err := xmclInst.InstallLibraries(ctx, nil, dotMC, fr); err != nil {
			return nil, fmt.Errorf("install fabric libs: %w", err)
		}
		resolvedID = id
	case LoaderForge:
		ver := plan.LoaderVersion
		if ver == "" {
			pr, err := forge.FetchPromotions(ctx, s.HTTP)
			if err != nil {
				return nil, err
			}
			ver = pr.LatestRecommended(plan.MCVersion)
			if ver == "" {
				return nil, fmt.Errorf("no Forge build for %s", plan.MCVersion)
			}
		}
		id, err := forge.Install(ctx, s.HTTP, s.Reporter, dotMC, javaPath, plan.MCVersion, ver)
		if err != nil {
			return nil, fmt.Errorf("install forge: %w", err)
		}
		resolvedID = id
	case LoaderNeoForge:
		ver := plan.LoaderVersion
		if ver == "" {
			all, err := neoforge.ListVersions(ctx, s.HTTP)
			if err != nil {
				return nil, err
			}
			matches := neoforge.VersionsForMC(all, plan.MCVersion)
			if len(matches) == 0 {
				return nil, fmt.Errorf("no NeoForge for %s", plan.MCVersion)
			}
			ver = matches[0]
		}
		id, err := neoforge.Install(ctx, s.HTTP, s.Reporter, dotMC, javaPath, ver)
		if err != nil {
			return nil, fmt.Errorf("install neoforge: %w", err)
		}
		resolvedID = id
	default:
		return nil, fmt.Errorf("unknown loader %q", plan.Loader)
	}

	res.VersionID = resolvedID

	// 5. Download the mods listed in the modpack manifest.
	if res.ManifestData != nil {
		if err := s.downloadManifestMods(ctx, dotMC, res.ManifestData); err != nil {
			return nil, fmt.Errorf("download modpack mods: %w", err)
		}
	}
	return res, nil
}

// downloadManifestMods fetches every CurseForge file from manifest.files
// concurrently and drops them into mods/.
func (s *Service) downloadManifestMods(ctx context.Context, dotMC string, m *curseforge.ModpackManifest) error {
	if len(m.Files) == 0 {
		return nil
	}
	ids := make([]int, 0, len(m.Files))
	for _, f := range m.Files {
		ids = append(ids, f.FileID)
	}
	// Batch in chunks of 256 — CF accepts up to 1000 but smaller is friendlier.
	var files []curseforge.File
	for chunk := range chunks(ids, 256) {
		batch, err := s.CF.GetFiles(ctx, chunk)
		if err != nil {
			return err
		}
		files = append(files, batch...)
	}
	modsDir := filepath.Join(dotMC, "mods")
	if err := ensureDir(modsDir); err != nil {
		return err
	}
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	errCh := make(chan error, len(files))
	for _, f := range files {
		f := f
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			url := curseforge.ResolveDownloadURL(f)
			dest := filepath.Join(modsDir, sanitizeName(f.FileName))
			if err := s.HTTP.Download(ctx, httpclient.DownloadOptions{
				URL: url, DestPath: dest, Size: f.FileLength,
				SHA1: pickSHA1(f.Hashes), Headers: map[string]string{"Referer": "https://www.curseforge.com/"},
				Reporter: s.Reporter, TaskID: f.FileName, Stage: progress.StageMods,
			}); err != nil {
				errCh <- fmt.Errorf("mod %s: %w", f.FileName, err)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func primaryLoader(ml []curseforge.ModLoaderManifest) (Modloader, string, bool) {
	pickFirst := len(ml) > 0
	for _, l := range ml {
		if l.Primary || pickFirst {
			loader, ver := splitLoaderID(l.ID)
			return loader, ver, true
		}
	}
	return "", "", false
}

// splitLoaderID parses "forge-47.2.0" → (LoaderForge, "47.2.0").
func splitLoaderID(id string) (Modloader, string) {
	idx := strings.IndexByte(id, '-')
	if idx < 0 {
		return Modloader(id), ""
	}
	prefix := strings.ToLower(id[:idx])
	ver := id[idx+1:]
	switch prefix {
	case "forge":
		return LoaderForge, ver
	case "fabric":
		return LoaderFabric, ver
	case "neoforge":
		return LoaderNeoForge, ver
	}
	return Modloader(prefix), ver
}

func pickSHA1(hashes []curseforge.Hash) string {
	for _, h := range hashes {
		if h.Algo == 1 {
			return h.Value
		}
	}
	return ""
}

func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}

// chunks yields successive slices of size at most n from xs.
func chunks(xs []int, n int) <-chan []int {
	ch := make(chan []int)
	go func() {
		defer close(ch)
		for i := 0; i < len(xs); i += n {
			j := i + n
			if j > len(xs) {
				j = len(xs)
			}
			ch <- xs[i:j]
		}
	}()
	return ch
}

func ensureDir(d string) error {
	return os.MkdirAll(d, 0o755)
}
