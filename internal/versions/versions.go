// Package versions enumerates locally-installed Minecraft profiles by
// scanning <folder>/versions/.
package versions

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/xmcl/version"
)

// Local describes one installed profile.
type Local struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	MainClass   string `json:"mainClass"`
	JavaMajor   int    `json:"javaMajor"`
	HasClient   bool   `json:"hasClient"`
	InheritedFrom string `json:"inheritedFrom,omitempty"`
}

// List returns every version currently installed under folder/versions/.
// Entries that fail to parse are skipped (and the parse error is suppressed
// since these are user-readable hints, not fatal data).
func List(folder string) ([]Local, error) {
	root := filepath.Join(folder, "versions")
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Local
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		l := Local{ID: id}
		jsonPath := version.VersionJSONPath(folder, id)
		if _, statErr := os.Stat(jsonPath); statErr != nil {
			continue
		}
		if r, parseErr := version.Parse(folder, id); parseErr == nil {
			l.Type = r.Type
			l.MainClass = r.MainClass
			l.JavaMajor = r.JavaVersion.MajorVersion
		}
		// Parent is recorded if the on-disk JSON has inheritsFrom (not the
		// resolved chain), so the UI can show the relationship.
		raw, _ := os.ReadFile(jsonPath)
		if rawParsed, perr := version.ParseFromBytes(folder, raw); perr == nil && rawParsed != nil {
			l.InheritedFrom = rawParsed.InheritsFrom
		}
		jarPath := version.VersionJARPath(folder, id)
		if st, statErr := os.Stat(jarPath); statErr == nil && st.Size() > 0 {
			l.HasClient = true
		}
		out = append(out, l)
	}
	return out, nil
}

// Service is the Wails-bound facade exposing version listing to the UI.
type Service struct {
	Folder string
}

func NewService(folder string) *Service { return &Service{Folder: folder} }

func (s *Service) List() ([]Local, error) { return List(s.Folder) }
