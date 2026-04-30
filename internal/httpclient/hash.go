package httpclient

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hash"
	"io"
)

type hashAccumulator struct {
	h hash.Hash
}

func newHash(kind string) *hashAccumulator {
	switch kind {
	case "sha1":
		return &hashAccumulator{h: sha1.New()}
	default:
		return &hashAccumulator{h: sha256.New()}
	}
}

func (h *hashAccumulator) Write(p []byte) (int, error) { return h.h.Write(p) }
func (h *hashAccumulator) Hex() string                  { return hex.EncodeToString(h.h.Sum(nil)) }

func decodeJSON(r io.Reader, out any) error {
	return json.NewDecoder(r).Decode(out)
}
