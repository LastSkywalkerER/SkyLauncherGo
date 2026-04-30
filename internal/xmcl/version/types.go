// Package version parses and resolves the per-version JSON manifest that
// describes a Minecraft client install (libraries, assets, JVM args).
//
// The schema follows Mojang's launcher format:
//   - legacy "minecraftArguments" string field (1.12 and below)
//   - modern split "arguments" with "game" and "jvm" arrays (1.13+)
//   - "inheritsFrom" lets modloader profiles (Fabric, Forge) layer on top
//     of a vanilla base; we resolve the chain transparently.
package version

import "encoding/json"

// Raw is the on-disk JSON exactly as Mojang ships it.
type Raw struct {
	ID                     string             `json:"id"`
	InheritsFrom           string             `json:"inheritsFrom,omitempty"`
	Type                   string             `json:"type,omitempty"`
	MainClass              string             `json:"mainClass,omitempty"`
	MinecraftArguments     string             `json:"minecraftArguments,omitempty"`
	Arguments              *Arguments         `json:"arguments,omitempty"`
	AssetIndex             *AssetIndex        `json:"assetIndex,omitempty"`
	Assets                 string             `json:"assets,omitempty"`
	Downloads              map[string]Download `json:"downloads,omitempty"`
	Libraries              []Library          `json:"libraries,omitempty"`
	JavaVersion            *JavaVersion       `json:"javaVersion,omitempty"`
	MinimumLauncherVersion int                `json:"minimumLauncherVersion,omitempty"`
	Logging                *Logging           `json:"logging,omitempty"`
	ReleaseTime            string             `json:"releaseTime,omitempty"`
	Time                   string             `json:"time,omitempty"`
}

// Arguments is the modern (1.13+) split argument structure.
//
// Each entry is either a plain string ("--foo") or an object with rules.
// We model it as []json.RawMessage so we can re-decode lazily.
type Arguments struct {
	Game []json.RawMessage `json:"game,omitempty"`
	JVM  []json.RawMessage `json:"jvm,omitempty"`
}

// AssetIndex points to the assets/indexes/<id>.json file.
type AssetIndex struct {
	ID        string `json:"id"`
	SHA1      string `json:"sha1"`
	Size      int64  `json:"size"`
	TotalSize int64  `json:"totalSize"`
	URL       string `json:"url"`
}

// Download describes a downloadable artifact (client.jar, mappings, etc).
type Download struct {
	URL  string `json:"url"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
}

// Library is one JAR dependency, possibly with native classifier.
type Library struct {
	Name      string             `json:"name"` // maven coords "group:artifact:version[:classifier]"
	URL       string             `json:"url,omitempty"`
	Downloads *LibraryDownloads  `json:"downloads,omitempty"`
	Natives   map[string]string  `json:"natives,omitempty"`   // legacy classifier map per OS
	Extract   *ExtractRule       `json:"extract,omitempty"`
	Rules     []Rule             `json:"rules,omitempty"`
}

// LibraryDownloads is the modern download metadata block.
type LibraryDownloads struct {
	Artifact    *Artifact            `json:"artifact,omitempty"`
	Classifiers map[string]*Artifact `json:"classifiers,omitempty"`
}

// Artifact references a single JAR file by path/url/sha1.
type Artifact struct {
	Path string `json:"path"`
	URL  string `json:"url"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
}

// ExtractRule lists path prefixes excluded from native zip extraction.
type ExtractRule struct {
	Exclude []string `json:"exclude,omitempty"`
}

// Rule is the OS/feature gate for arguments and libraries.
type Rule struct {
	Action   string                 `json:"action"` // "allow" | "disallow"
	OS       *OSRule                `json:"os,omitempty"`
	Features map[string]bool        `json:"features,omitempty"`
}

// OSRule constrains by OS name/version/arch.
type OSRule struct {
	Name    string `json:"name,omitempty"`    // "windows" | "osx" | "linux"
	Version string `json:"version,omitempty"` // regex
	Arch    string `json:"arch,omitempty"`    // "x86" | "x86_64" | "arm64"
}

// JavaVersion is the suggested Java major from Mojang.
type JavaVersion struct {
	Component    string `json:"component"`
	MajorVersion int    `json:"majorVersion"`
}

// Logging carries log4j XML config for older versions.
type Logging struct {
	Client *LoggingClient `json:"client,omitempty"`
}

type LoggingClient struct {
	Argument string                 `json:"argument"`
	File     map[string]any         `json:"file"`
	Type     string                 `json:"type"`
}
