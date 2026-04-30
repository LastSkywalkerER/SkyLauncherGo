package updater

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestVerifySignatureRoundTrip(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	digest := "deadbeefcafebabe" // arbitrary hex digest stand-in
	sig := ed25519.Sign(priv, []byte(digest))
	if !verifySignature(pub, digest, base64.StdEncoding.EncodeToString(sig)) {
		t.Error("expected valid signature to verify")
	}
	if verifySignature(pub, "wrong", base64.StdEncoding.EncodeToString(sig)) {
		t.Error("expected mismatched digest to fail verification")
	}
}

func TestPickArtifact(t *testing.T) {
	files := []ManifestFile{
		{Platform: "windows", Arch: "amd64", URL: "win"},
		{Platform: "darwin", Arch: "universal", URL: "mac"},
		{Platform: "linux", Arch: "amd64", URL: "lin"},
	}
	if got := pickArtifact(files, "darwin", "arm64"); got == nil || got.URL != "mac" {
		t.Errorf("darwin arm64 should match universal, got %+v", got)
	}
	if got := pickArtifact(files, "windows", "amd64"); got == nil || got.URL != "win" {
		t.Errorf("windows amd64 should match exact, got %+v", got)
	}
	if got := pickArtifact(files, "freebsd", "amd64"); got != nil {
		t.Errorf("expected nil for unsupported platform, got %+v", got)
	}
}
