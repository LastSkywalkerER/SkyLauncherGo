package java

import "github.com/LastSkywalkerER/SkyLauncherGo/internal/xmcl/version"

// MajorFor returns the suggested Java major version for a resolved
// Minecraft profile, falling back to a heuristic if Mojang didn't ship one.
func MajorFor(r *version.Resolved) int {
	if r != nil && r.JavaVersion.MajorVersion > 0 {
		return r.JavaVersion.MajorVersion
	}
	return 17
}
