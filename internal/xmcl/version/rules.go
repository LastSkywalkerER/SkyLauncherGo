package version

import (
	"runtime"
	"strings"
)

// HostOS is the Mojang-style OS name for the current platform.
func HostOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "osx"
	case "windows":
		return "windows"
	case "linux":
		return "linux"
	}
	return runtime.GOOS
}

// HostArch is the Mojang-style architecture name.
func HostArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "arm64"
	case "386":
		return "x86"
	}
	return runtime.GOARCH
}

// MatchRules evaluates a list of rules against the host environment plus a
// feature map (e.g. "is_demo_user") and reports whether the gated entry
// should be applied.
func MatchRules(rules []Rule, features map[string]bool) bool {
	if len(rules) == 0 {
		return true
	}
	allowed := false
	for _, r := range rules {
		if !ruleAppliesToHost(r, features) {
			continue
		}
		switch strings.ToLower(r.Action) {
		case "allow":
			allowed = true
		case "disallow":
			return false
		}
	}
	return allowed
}

func ruleAppliesToHost(r Rule, features map[string]bool) bool {
	if r.OS != nil {
		if r.OS.Name != "" && r.OS.Name != HostOS() {
			return false
		}
		if r.OS.Arch != "" && r.OS.Arch != HostArch() {
			return false
		}
		// Version regex check is omitted; matches the legacy launcher's
		// behaviour of trusting OS name + arch alone.
	}
	for k, want := range r.Features {
		got := features[k]
		if got != want {
			return false
		}
	}
	return true
}

// LibraryNativeClassifier returns the native classifier (e.g. "natives-linux")
// that applies on the current host, or "" if this library has no native part.
func LibraryNativeClassifier(l Library) string {
	if len(l.Natives) == 0 {
		return ""
	}
	tpl, ok := l.Natives[HostOS()]
	if !ok {
		return ""
	}
	// Mojang uses ${arch} placeholder = "32"|"64"
	arch := "64"
	if runtime.GOARCH == "386" {
		arch = "32"
	}
	return strings.ReplaceAll(tpl, "${arch}", arch)
}
