// Package config persists user-tunable launcher settings to a JSON file.
//
// The on-disk format is intentionally compatible with the Electron-era
// `electron-store` schema so existing users transition without re-configuring.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/paths"
)

const fileName = "config.json"

// JavaConfig matches the Electron DTO (`src/shared/dtos/config.dto.ts`).
type JavaConfig struct {
	Memory int    `json:"memory"`
	Path   string `json:"path,omitempty"`
}

// UserProfile is the cached MS-auth or offline-nick profile.
type UserProfile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	Type         string `json:"type"` // "microsoft" | "offline"
	ExpiresAt    int64  `json:"expiresAt,omitempty"`
}

// Data is the full persisted document.
type Data struct {
	Locale      string      `json:"locale"`
	Java        JavaConfig  `json:"java"`
	User        UserProfile `json:"user"`
	LastInstance string     `json:"lastInstance,omitempty"`
}

// Defaults returns the seed configuration used on first run.
func Defaults() Data {
	return Data{
		Locale: "en",
		Java: JavaConfig{
			Memory: 4096,
		},
	}
}

// Load reads the on-disk config, populating defaults for missing fields.
func Load() (Data, error) {
	d := Defaults()
	path, err := filePath()
	if err != nil {
		return d, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return Defaults(), err
	}
	if d.Java.Memory <= 0 {
		d.Java.Memory = Defaults().Java.Memory
	}
	if d.Locale == "" {
		d.Locale = Defaults().Locale
	}
	return d, nil
}

// Save atomically writes the config back to disk.
func Save(d Data) error {
	path, err := filePath()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func filePath() (string, error) {
	dir, err := paths.UserDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

// Service is the Wails-bound facade exposed to the frontend.
type Service struct {
	mu   sync.RWMutex
	data Data
}

func NewService(initial Data) *Service {
	return &Service{data: initial}
}

func (s *Service) Get() Data {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

func (s *Service) SetLocale(locale string) error {
	s.mu.Lock()
	s.data.Locale = locale
	d := s.data
	s.mu.Unlock()
	return Save(d)
}

func (s *Service) SetJava(j JavaConfig) error {
	s.mu.Lock()
	s.data.Java = j
	d := s.data
	s.mu.Unlock()
	return Save(d)
}

func (s *Service) SetUser(u UserProfile) error {
	s.mu.Lock()
	s.data.User = u
	d := s.data
	s.mu.Unlock()
	return Save(d)
}

func (s *Service) SetLastInstance(id string) error {
	s.mu.Lock()
	s.data.LastInstance = id
	d := s.data
	s.mu.Unlock()
	return Save(d)
}
