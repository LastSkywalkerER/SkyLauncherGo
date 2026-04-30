// Package java provisions a Java Runtime Environment for launching MC.
//
// We use the Adoptium / Temurin REST API as the JRE source. JREs live under
// {userdata}/jre/{major}/ and the resolver picks one based on Mojang's
// `javaVersion.majorVersion` field on the version manifest.
package java

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/httpclient"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/paths"
	"github.com/LastSkywalkerER/SkyLauncherGo/internal/progress"
)

const adoptiumBase = "https://api.adoptium.net/v3"

// Provider knows how to ensure a JRE of a given major version is on disk.
type Provider struct {
	HTTP     *httpclient.Client
	Reporter progress.Reporter
}

func NewProvider(c *httpclient.Client, rep progress.Reporter) *Provider {
	if rep == nil {
		rep = progress.Noop{}
	}
	return &Provider{HTTP: c, Reporter: rep}
}

// EnsureJRE returns the absolute path to a `java(.exe)` for the given major
// version, downloading and extracting an Adoptium JRE if needed.
func (p *Provider) EnsureJRE(ctx context.Context, major int) (string, error) {
	if major <= 0 {
		major = 17
	}
	root, err := paths.JREDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, fmt.Sprintf("%d", major))
	if path, ok := findJavaUnder(dir); ok {
		return path, nil
	}

	link, err := p.resolveBinary(ctx, major)
	if err != nil {
		return "", err
	}
	tmpDir, err := os.MkdirTemp("", "skylauncher-jre-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	archive := filepath.Join(tmpDir, "jre"+ext(link.URL))
	if err := p.HTTP.Download(ctx, httpclient.DownloadOptions{
		URL: link.URL, DestPath: archive, Size: link.Size, SHA256: link.SHA256,
		Reporter: p.Reporter, Stage: progress.StageJRE, TaskID: fmt.Sprintf("jre-%d", major),
	}); err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if strings.HasSuffix(link.URL, ".zip") {
		if err := unzip(archive, dir); err != nil {
			return "", err
		}
	} else {
		if err := untargz(archive, dir); err != nil {
			return "", err
		}
	}
	if path, ok := findJavaUnder(dir); ok {
		return path, nil
	}
	return "", errors.New("java executable not found after extraction")
}

type binaryLink struct {
	URL    string
	SHA256 string
	Size   int64
}

func (p *Provider) resolveBinary(ctx context.Context, major int) (binaryLink, error) {
	osName := adoptiumOS()
	arch := adoptiumArch()
	url := fmt.Sprintf("%s/assets/feature_releases/%d/ga?os=%s&architecture=%s&image_type=jre&jvm_impl=hotspot&heap_size=normal&vendor=eclipse&page_size=1",
		adoptiumBase, major, osName, arch)

	var releases []struct {
		Binaries []struct {
			Package struct {
				Link     string `json:"link"`
				Checksum string `json:"checksum"`
				Size     int64  `json:"size"`
			} `json:"package"`
		} `json:"binaries"`
	}
	if err := p.HTTP.GetJSON(ctx, url, nil, &releases); err != nil {
		return binaryLink{}, fmt.Errorf("adoptium %d: %w", major, err)
	}
	for _, r := range releases {
		for _, b := range r.Binaries {
			if b.Package.Link != "" {
				return binaryLink{URL: b.Package.Link, SHA256: b.Package.Checksum, Size: b.Package.Size}, nil
			}
		}
	}
	return binaryLink{}, fmt.Errorf("no adoptium release for major %d on %s/%s", major, osName, arch)
}

func adoptiumOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "mac"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

func adoptiumArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "aarch64"
	}
	return runtime.GOARCH
}

func ext(url string) string {
	if strings.HasSuffix(url, ".zip") {
		return ".zip"
	}
	return ".tar.gz"
}

// findJavaUnder walks dir looking for a `bin/java(.exe)` and returns it.
func findJavaUnder(dir string) (string, bool) {
	exe := "java"
	if runtime.GOOS == "windows" {
		exe = "java.exe"
	}
	var found string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Base(path) == exe && filepath.Base(filepath.Dir(path)) == "bin" {
			found = path
			return io.EOF
		}
		return nil
	})
	return found, found != ""
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		out := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(filepath.Clean(out)+string(os.PathSeparator), filepath.Clean(dest)+string(os.PathSeparator)) {
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
		w, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(w, rc)
		rc.Close()
		w.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func untargz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		out := filepath.Join(dest, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(out)+string(os.PathSeparator), filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(out, os.FileMode(hdr.Mode)|0o111)
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return err
			}
			w, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)|0o100)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return err
			}
			w.Close()
		case tar.TypeSymlink, tar.TypeLink:
			_ = os.Symlink(hdr.Linkname, out)
		}
	}
}
