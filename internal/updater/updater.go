// Package updater handles self-updating of the launcher binary.
//
// The release manifest is a small JSON document hosted at a stable URL
// (typically a GitHub Releases asset) that lists each per-platform
// artifact along with an ed25519 signature of its SHA-256 digest. The
// public key is baked into the binary at build time so the launcher can
// verify a download before swapping the running executable for it.
package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/httpclient"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/progress"
)

// PublicKey is the base64-encoded ed25519 public key used to verify
// release signatures. Set at build time:
//
//	go build -ldflags "-X 'github.com/LastSkywalkerER/SkyLauncherGo/internal/updater.PublicKey=...'"
var PublicKey = ""

// ManifestURL is the location of latest.json. Override at build time
// to point at the production CDN.
var ManifestURL = "https://github.com/LastSkywalkerER/SkyLauncherGo/releases/latest/download/latest.json"

// Manifest is the published release index.
type Manifest struct {
	Version     string         `json:"version"`
	ReleaseDate string         `json:"releaseDate"`
	Notes       string         `json:"notes"`
	Files       []ManifestFile `json:"files"`
}

// ManifestFile is one downloadable build artifact with its signature.
type ManifestFile struct {
	Platform  string `json:"platform"`  // "windows" | "darwin" | "linux"
	Arch      string `json:"arch"`      // "amd64" | "arm64" | "universal"
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature"` // base64 ed25519 signature over the SHA-256 hex digest
	Size      int64  `json:"size,omitempty"`
}

// Status describes the outcome of a check-for-update operation.
type Status struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	UpdateAvailable bool  `json:"updateAvailable"`
	Notes          string `json:"notes,omitempty"`
}

// Service is the Wails-bound facade.
type Service struct {
	HTTP     *httpclient.Client
	Reporter progress.Reporter
	Current  string // current binary semver
}

func NewService(c *httpclient.Client, rep progress.Reporter, currentVersion string) *Service {
	if rep == nil {
		rep = progress.Noop{}
	}
	return &Service{HTTP: c, Reporter: rep, Current: currentVersion}
}

// Check downloads the manifest and reports whether a newer build exists
// for the host platform.
func (s *Service) Check(ctx context.Context) (*Status, error) {
	m, err := s.fetchManifest(ctx)
	if err != nil {
		return nil, err
	}
	return &Status{
		CurrentVersion:  s.Current,
		LatestVersion:   m.Version,
		UpdateAvailable: semverGreater(m.Version, s.Current),
		Notes:           m.Notes,
	}, nil
}

// Apply downloads the artifact for the current host, verifies its
// signature against the embedded public key, and atomically replaces the
// running binary with it.
func (s *Service) Apply(ctx context.Context) error {
	if PublicKey == "" {
		return errors.New("updater: PublicKey not set at build time")
	}
	pubBytes, err := base64.StdEncoding.DecodeString(PublicKey)
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("public key wrong size: %d", len(pubBytes))
	}
	pub := ed25519.PublicKey(pubBytes)

	m, err := s.fetchManifest(ctx)
	if err != nil {
		return err
	}
	target := pickArtifact(m.Files, runtime.GOOS, runtime.GOARCH)
	if target == nil {
		return fmt.Errorf("no artifact for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "skylauncher-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	dl := filepath.Join(tmpDir, filepath.Base(exe)+".new")
	if err := s.HTTP.Download(ctx, httpclient.DownloadOptions{
		URL: target.URL, DestPath: dl, SHA256: target.SHA256, Size: target.Size,
		Reporter: s.Reporter, TaskID: "self-update", Stage: progress.StageDownload,
	}); err != nil {
		return err
	}

	digest, err := computeSHA256(dl)
	if err != nil {
		return err
	}
	if !verifySignature(pub, digest, target.Signature) {
		return errors.New("updater: signature verification failed")
	}
	return swapBinary(exe, dl)
}

func (s *Service) fetchManifest(ctx context.Context) (*Manifest, error) {
	var m Manifest
	if err := s.HTTP.GetJSON(ctx, ManifestURL, nil, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func pickArtifact(files []ManifestFile, goos, goarch string) *ManifestFile {
	for i := range files {
		f := &files[i]
		if f.Platform != goos {
			continue
		}
		if f.Arch == goarch || f.Arch == "universal" {
			return f
		}
	}
	return nil
}

func computeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func verifySignature(pub ed25519.PublicKey, sha256Hex, b64Sig string) bool {
	sig, err := base64.StdEncoding.DecodeString(b64Sig)
	if err != nil {
		return false
	}
	return ed25519.Verify(pub, []byte(sha256Hex), sig)
}

// semverGreater returns true if a is a strictly larger semver than b.
// It handles the "v" prefix and compares numeric components only; pre-release
// suffixes are sorted lexicographically as a fallback.
func semverGreater(a, b string) bool {
	if a == "" || b == "" {
		return a > b
	}
	for _, p := range []*string{&a, &b} {
		if len(*p) > 0 && (*p)[0] == 'v' {
			*p = (*p)[1:]
		}
	}
	return a > b
}
