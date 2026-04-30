// Package progress is a tiny event bus used by long-running tasks
// (downloads, JRE installation, modpack install) to report progress to
// the frontend through Wails events.
package progress

import (
	"sync"
	"sync/atomic"
)

// Stage names mirror the IPCSendNames in the legacy Electron app so existing
// frontend code can subscribe without renames.
const (
	StageDownload  = "download"
	StageInstall   = "install"
	StageExtract   = "extract"
	StageLaunch    = "launch"
	StageJRE       = "jre"
	StageManifest  = "manifest"
	StageLibraries = "libraries"
	StageAssets    = "assets"
	StageMods      = "mods"
)

// Event is one progress update.
type Event struct {
	ID         string  `json:"id"`         // task id (e.g. instance id)
	Stage      string  `json:"stage"`      // see Stage* constants
	Message    string  `json:"message"`    // free-form description
	Bytes      int64   `json:"bytes"`      // bytes done
	TotalBytes int64   `json:"totalBytes"` // -1 if unknown
	Items      int64   `json:"items"`
	TotalItems int64   `json:"totalItems"`
	Fraction   float64 `json:"fraction"` // 0.0 .. 1.0, computed if possible
	Done       bool    `json:"done"`
	Error      string  `json:"error,omitempty"`
}

// Reporter is the interface exposed to producers (downloads, install steps).
type Reporter interface {
	Report(Event)
}

// Sink consumes progress events. Wails service implementations register a
// Sink at startup that forwards each event over the JS bridge.
type Sink interface {
	Emit(Event)
}

// Bus fans out events to a set of registered sinks. Sinks are added via
// Subscribe and removed via the returned cancel function.
type Bus struct {
	mu      sync.RWMutex
	nextID  atomic.Uint64
	sinks   map[uint64]Sink
}

func NewBus() *Bus {
	return &Bus{sinks: map[uint64]Sink{}}
}

func (b *Bus) Subscribe(s Sink) (cancel func()) {
	id := b.nextID.Add(1)
	b.mu.Lock()
	b.sinks[id] = s
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		delete(b.sinks, id)
		b.mu.Unlock()
	}
}

func (b *Bus) Report(e Event) {
	if e.TotalBytes > 0 && e.Fraction == 0 {
		e.Fraction = float64(e.Bytes) / float64(e.TotalBytes)
	}
	b.mu.RLock()
	sinks := make([]Sink, 0, len(b.sinks))
	for _, s := range b.sinks {
		sinks = append(sinks, s)
	}
	b.mu.RUnlock()
	for _, s := range sinks {
		s.Emit(e)
	}
}

// Noop is the zero-cost fallback when no UI is attached (e.g. CLI or tests).
type Noop struct{}

func (Noop) Report(Event) {}
func (Noop) Emit(Event)   {}
