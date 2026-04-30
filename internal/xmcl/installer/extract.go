package installer

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// extractZipExcluding unpacks src into destDir but skips entries whose path
// starts with any prefix in `exclude` (used for native libs to drop META-INF).
func extractZipExcluding(src, destDir string, exclude []string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	for _, f := range r.File {
		if shouldExclude(f.Name, exclude) {
			continue
		}
		if f.FileInfo().IsDir() {
			continue
		}
		// Block path traversal: zip entries shouldn't escape destDir.
		clean := filepath.Clean(f.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			continue
		}
		out := filepath.Join(destDir, clean)
		if !strings.HasPrefix(filepath.Clean(out)+string(os.PathSeparator), filepath.Clean(destDir)+string(os.PathSeparator)) {
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

func shouldExclude(name string, exclude []string) bool {
	for _, p := range exclude {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}
