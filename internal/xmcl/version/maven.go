package version

import (
	"path/filepath"
	"strings"
)

// MavenPath converts a maven coordinate "group:artifact:version[:classifier][@ext]"
// into the canonical relative jar path under <libraries>/.
//
// Examples:
//
//	"org.lwjgl:lwjgl:3.3.1"             → "org/lwjgl/lwjgl/3.3.1/lwjgl-3.3.1.jar"
//	"net.minecraft:client:1.20.1:slim"  → "net/minecraft/client/1.20.1/client-1.20.1-slim.jar"
//	"org.junit:junit:5.0@pom"           → "org/junit/junit/5.0/junit-5.0.pom"
func MavenPath(coord string) string {
	ext := "jar"
	if i := strings.LastIndex(coord, "@"); i > 0 {
		ext = coord[i+1:]
		coord = coord[:i]
	}
	parts := strings.Split(coord, ":")
	if len(parts) < 3 {
		return ""
	}
	group, artifact, ver := parts[0], parts[1], parts[2]
	classifier := ""
	if len(parts) >= 4 {
		classifier = parts[3]
	}
	groupPath := strings.ReplaceAll(group, ".", "/")
	file := artifact + "-" + ver
	if classifier != "" {
		file += "-" + classifier
	}
	file += "." + ext
	return filepath.ToSlash(filepath.Join(groupPath, artifact, ver, file))
}
