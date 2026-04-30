package version

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Resolved is a version manifest with all `inheritsFrom` chains merged.
type Resolved struct {
	ID                 string
	Type               string
	MainClass          string
	AssetIndex         AssetIndex
	Assets             string
	ClientDownload     Download
	Libraries          []Library
	JavaVersion        JavaVersion
	MinecraftArguments string            // legacy single-string args
	GameArguments      []json.RawMessage // modern args
	JVMArguments       []json.RawMessage
	Logging            *Logging

	// Folder is the .minecraft root used to locate child version JSON files.
	Folder string
}

// Parse loads <folder>/versions/<id>/<id>.json and resolves any inheritsFrom
// chain by merging parent manifests in order.
func Parse(folder, id string) (*Resolved, error) {
	if folder == "" || id == "" {
		return nil, errors.New("version.Parse: folder and id required")
	}
	chain, err := loadChain(folder, id, map[string]bool{})
	if err != nil {
		return nil, err
	}
	res := &Resolved{ID: id, Folder: folder}
	// Walk root → leaf so leaf overrides win.
	for i := len(chain) - 1; i >= 0; i-- {
		merge(res, chain[i])
	}
	if res.MainClass == "" {
		return nil, fmt.Errorf("version %s: missing mainClass after resolution", id)
	}
	return res, nil
}

// ParseFromBytes is a helper for tests; folder is recorded but no IO done.
func ParseFromBytes(folder string, data []byte) (*Raw, error) {
	var r Raw
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// VersionJSONPath returns the canonical path to a version manifest on disk.
func VersionJSONPath(folder, id string) string {
	return filepath.Join(folder, "versions", id, id+".json")
}

// VersionJARPath returns the canonical path to the version's client jar.
func VersionJARPath(folder, id string) string {
	return filepath.Join(folder, "versions", id, id+".jar")
}

func loadChain(folder, id string, seen map[string]bool) ([]*Raw, error) {
	if seen[id] {
		return nil, fmt.Errorf("inheritsFrom cycle at %s", id)
	}
	seen[id] = true

	path := VersionJSONPath(folder, id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read version json %s: %w", path, err)
	}
	var r Raw
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse version json %s: %w", path, err)
	}
	chain := []*Raw{&r}
	if r.InheritsFrom != "" {
		parent, err := loadChain(folder, r.InheritsFrom, seen)
		if err != nil {
			return nil, err
		}
		chain = append(chain, parent...)
	}
	return chain, nil
}

func merge(res *Resolved, r *Raw) {
	if r.Type != "" {
		res.Type = r.Type
	}
	if r.MainClass != "" {
		res.MainClass = r.MainClass
	}
	if r.AssetIndex != nil {
		res.AssetIndex = *r.AssetIndex
	}
	if r.Assets != "" {
		res.Assets = r.Assets
	}
	if d, ok := r.Downloads["client"]; ok {
		res.ClientDownload = d
	}
	if r.JavaVersion != nil {
		res.JavaVersion = *r.JavaVersion
	}
	if r.MinecraftArguments != "" {
		res.MinecraftArguments = r.MinecraftArguments
	}
	if r.Arguments != nil {
		res.GameArguments = append(res.GameArguments, r.Arguments.Game...)
		res.JVMArguments = append(res.JVMArguments, r.Arguments.JVM...)
	}
	if r.Logging != nil {
		res.Logging = r.Logging
	}
	// Library merging: child entries override by maven group:artifact key.
	if len(r.Libraries) > 0 {
		res.Libraries = mergeLibraries(res.Libraries, r.Libraries)
	}
}

func mergeLibraries(existing, incoming []Library) []Library {
	keyOf := func(name string) string {
		// "group:artifact:version[:classifier]" → "group:artifact"
		i := indexByte(name, ':')
		if i < 0 {
			return name
		}
		j := indexByte(name[i+1:], ':')
		if j < 0 {
			return name
		}
		return name[:i+1+j]
	}
	have := map[string]int{}
	merged := make([]Library, 0, len(existing)+len(incoming))
	for i, l := range existing {
		have[keyOf(l.Name)] = i
		merged = append(merged, l)
	}
	for _, l := range incoming {
		k := keyOf(l.Name)
		if idx, ok := have[k]; ok {
			merged[idx] = l
			continue
		}
		have[k] = len(merged)
		merged = append(merged, l)
	}
	return merged
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
