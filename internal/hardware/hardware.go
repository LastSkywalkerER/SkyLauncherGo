// Package hardware reports basic host capabilities to the UI so it can
// suggest sensible defaults (e.g. JVM heap size, parallel download count).
package hardware

import (
	"runtime"

	"github.com/pbnjay/memory"
)

// Info describes the host system.
type Info struct {
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	CPUCount   int    `json:"cpuCount"`
	TotalRAMMB uint64 `json:"totalRamMB"`
}

// Service is the Wails-bound facade.
type Service struct{}

func NewService() *Service { return &Service{} }

// Get returns the host info snapshot.
func (s *Service) Get() Info {
	return Info{
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		CPUCount:   runtime.NumCPU(),
		TotalRAMMB: memory.TotalMemory() / (1024 * 1024),
	}
}
