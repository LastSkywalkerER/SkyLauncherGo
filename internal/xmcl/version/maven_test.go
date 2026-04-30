package version

import "testing"

func TestMavenPath(t *testing.T) {
	cases := []struct {
		coord, want string
	}{
		{"org.lwjgl:lwjgl:3.3.1", "org/lwjgl/lwjgl/3.3.1/lwjgl-3.3.1.jar"},
		{"net.minecraft:client:1.20.1:slim", "net/minecraft/client/1.20.1/client-1.20.1-slim.jar"},
		{"com.example:lib:1.0@pom", "com/example/lib/1.0/lib-1.0.pom"},
		{"net.fabricmc:fabric-loader:0.15.7", "net/fabricmc/fabric-loader/0.15.7/fabric-loader-0.15.7.jar"},
	}
	for _, c := range cases {
		if got := MavenPath(c.coord); got != c.want {
			t.Errorf("MavenPath(%q) = %q, want %q", c.coord, got, c.want)
		}
	}
}

func TestMavenPathInvalid(t *testing.T) {
	if got := MavenPath("not-a-coord"); got != "" {
		t.Errorf("expected empty for invalid coord, got %q", got)
	}
}
