// Package instances tracks per-modpack `.minecraft` trees and the live MC
// processes they spawn. Several instances may run simultaneously.
package instances

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Instance is the on-disk descriptor stored next to .minecraft as instance.json.
type Instance struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	MCVersion  string    `json:"mcVersion"`
	Loader     string    `json:"loader,omitempty"`
	LoaderVer  string    `json:"loaderVersion,omitempty"`
	VersionID  string    `json:"versionId"` // resolved id used to launch
	JavaPath   string    `json:"javaPath,omitempty"`
	HeapMB     int       `json:"heapMB,omitempty"`
	IconURL    string    `json:"iconUrl,omitempty"`
	Modpack    string    `json:"modpack,omitempty"`    // CF mod id reference
	CreatedAt  time.Time `json:"createdAt"`
	LastPlayed time.Time `json:"lastPlayed,omitempty"`
}

// Manager owns the instance directory and tracks running processes.
type Manager struct {
	root string

	mu      sync.Mutex
	running map[string]*exec.Cmd
}

func NewManager(root string) (*Manager, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Manager{root: root, running: map[string]*exec.Cmd{}}, nil
}

// Root returns the directory under which all instances live.
func (m *Manager) Root() string { return m.root }

// Dir returns the directory for one instance ID.
func (m *Manager) Dir(id string) string { return filepath.Join(m.root, id) }

// MinecraftDir returns the .minecraft folder for the instance.
func (m *Manager) MinecraftDir(id string) string {
	return filepath.Join(m.Dir(id), ".minecraft")
}

// Create allocates a fresh instance directory and writes its descriptor.
func (m *Manager) Create(name, mcVersion string) (*Instance, error) {
	id := uuid.New().String()
	inst := &Instance{
		ID:        id,
		Name:      name,
		MCVersion: mcVersion,
		CreatedAt: time.Now().UTC(),
	}
	if err := os.MkdirAll(m.MinecraftDir(id), 0o755); err != nil {
		return nil, err
	}
	if err := m.write(inst); err != nil {
		return nil, err
	}
	return inst, nil
}

// Save persists changes to an instance descriptor.
func (m *Manager) Save(inst *Instance) error {
	if inst == nil || inst.ID == "" {
		return errors.New("save: instance id required")
	}
	return m.write(inst)
}

// Get returns one instance by id.
func (m *Manager) Get(id string) (*Instance, error) {
	return m.read(id)
}

// List walks the instance root, returning every readable descriptor.
func (m *Manager) List() ([]Instance, error) {
	entries, err := os.ReadDir(m.root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]Instance, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		inst, err := m.read(e.Name())
		if err != nil {
			continue
		}
		out = append(out, *inst)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastPlayed.After(out[j].LastPlayed) })
	return out, nil
}

// Delete removes an instance directory tree. If keepWorlds is true, the
// `.minecraft/saves` folder is moved to <root>/_saved/<id> first so the
// player doesn't lose progress.
func (m *Manager) Delete(id string, keepWorlds bool) error {
	if m.IsRunning(id) {
		return errors.New("delete: instance is running")
	}
	dir := m.Dir(id)
	if keepWorlds {
		saves := filepath.Join(dir, ".minecraft", "saves")
		if st, err := os.Stat(saves); err == nil && st.IsDir() {
			backup := filepath.Join(m.root, "_saved", id, "saves")
			_ = os.MkdirAll(filepath.Dir(backup), 0o755)
			_ = os.Rename(saves, backup)
		}
	}
	return os.RemoveAll(dir)
}

// AttachProcess records a freshly started MC process so callers can later
// stop or query it. Returns an error if the instance already has a live process.
func (m *Manager) AttachProcess(id string, cmd *exec.Cmd) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, busy := m.running[id]; busy {
		return errors.New("instance is already running")
	}
	m.running[id] = cmd
	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		delete(m.running, id)
		m.mu.Unlock()
	}()
	return nil
}

// Stop sends a kill signal to the running MC process for the instance, if any.
func (m *Manager) Stop(ctx context.Context, id string) error {
	m.mu.Lock()
	cmd, ok := m.running[id]
	m.mu.Unlock()
	if !ok || cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

// IsRunning reports whether an MC process is currently attached for id.
func (m *Manager) IsRunning(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.running[id]
	return ok
}

// RunningIDs returns the ids of every currently-attached process.
func (m *Manager) RunningIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.running))
	for k := range m.running {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (m *Manager) descriptorPath(id string) string {
	return filepath.Join(m.Dir(id), "instance.json")
}

func (m *Manager) write(inst *Instance) error {
	if err := os.MkdirAll(m.Dir(inst.ID), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(inst, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.descriptorPath(inst.ID), raw, 0o644)
}

func (m *Manager) read(id string) (*Instance, error) {
	raw, err := os.ReadFile(m.descriptorPath(id))
	if err != nil {
		return nil, fmt.Errorf("instance %s: %w", id, err)
	}
	var inst Instance
	if err := json.Unmarshal(raw, &inst); err != nil {
		return nil, err
	}
	return &inst, nil
}
