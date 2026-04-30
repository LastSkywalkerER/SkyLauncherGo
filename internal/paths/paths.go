// Package paths resolves OS-specific application directories.
//
// We deliberately use the same locations as Electron's `app.getPath('userData')`
// so that an existing Electron-era SkyLauncher installation can be migrated
// in-place without copying files.
package paths

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// AppName is the user-visible directory name. It must match what
// the legacy Electron app used so existing config.json is picked up.
const AppName = "SkyLauncher"

// UserDataDir returns the canonical per-user app data directory and ensures
// it exists. On Windows: %APPDATA%/SkyLauncher. On macOS:
// ~/Library/Application Support/SkyLauncher. On Linux: ~/.config/SkyLauncher.
func UserDataDir() (string, error) {
	dir, err := userBase()
	if err != nil {
		return "", err
	}
	full := filepath.Join(dir, AppName)
	if err := os.MkdirAll(full, 0o755); err != nil {
		return "", err
	}
	return full, nil
}

// InstancesDir returns the directory under which per-modpack `.minecraft`
// trees are kept.
func InstancesDir() (string, error) {
	base, err := UserDataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "instances")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// JREDir returns the directory where downloaded JREs live, organised by
// major version (`<dir>/17`, `<dir>/21`).
func JREDir() (string, error) {
	base, err := UserDataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "jre")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// CacheDir returns a transient working directory used for downloads and zip
// extraction. Callers may delete its contents at any time.
func CacheDir() (string, error) {
	base, err := UserDataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "cache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func userBase() (string, error) {
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("APPDATA"); v != "" {
			return v, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "AppData", "Roaming"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support"), nil
	case "linux":
		if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
			return v, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config"), nil
	}
	return "", errors.New("unsupported platform")
}
