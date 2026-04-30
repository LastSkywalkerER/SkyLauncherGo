package version

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeVersionJSON(t *testing.T, dir, id string, raw Raw) {
	t.Helper()
	path := VersionJSONPath(dir, id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseInheritsFrom(t *testing.T) {
	dir := t.TempDir()
	writeVersionJSON(t, dir, "1.20.1", Raw{
		ID: "1.20.1", Type: "release", MainClass: "net.minecraft.client.main.Main",
		Assets: "5",
		AssetIndex: &AssetIndex{ID: "5", URL: "https://example/assets.json", SHA1: "deadbeef"},
		Downloads: map[string]Download{"client": {URL: "https://example/client.jar", SHA1: "cafe", Size: 100}},
		Libraries: []Library{{Name: "org.example:base:1.0"}},
		JavaVersion: &JavaVersion{Component: "java-runtime-gamma", MajorVersion: 17},
	})
	writeVersionJSON(t, dir, "1.20.1-fabric", Raw{
		ID: "1.20.1-fabric", InheritsFrom: "1.20.1",
		MainClass: "net.fabricmc.loader.impl.launch.knot.KnotClient",
		Libraries: []Library{
			{Name: "net.fabricmc:fabric-loader:0.15.7"},
			{Name: "org.example:base:2.0"}, // overrides the parent base library
		},
	})

	r, err := Parse(dir, "1.20.1-fabric")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.MainClass != "net.fabricmc.loader.impl.launch.knot.KnotClient" {
		t.Errorf("child mainClass not preserved: %s", r.MainClass)
	}
	if r.AssetIndex.ID != "5" {
		t.Errorf("inherited assetIndex missing: %+v", r.AssetIndex)
	}
	if r.JavaVersion.MajorVersion != 17 {
		t.Errorf("inherited javaVersion missing")
	}
	if r.ClientDownload.URL != "https://example/client.jar" {
		t.Errorf("inherited client download missing")
	}
	// Library overlay: child "base:2.0" replaces parent's "base:1.0" and a
	// new fabric-loader entry is appended.
	if len(r.Libraries) != 2 {
		t.Fatalf("expected 2 libs after merge, got %d", len(r.Libraries))
	}
	foundBase := false
	for _, l := range r.Libraries {
		if l.Name == "org.example:base:2.0" {
			foundBase = true
		}
		if l.Name == "org.example:base:1.0" {
			t.Errorf("parent base library should have been replaced")
		}
	}
	if !foundBase {
		t.Errorf("child base library missing after merge")
	}
}
