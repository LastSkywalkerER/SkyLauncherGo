package auth

import (
	"errors"
	"regexp"

	"github.com/google/uuid"
)

var nickPattern = regexp.MustCompile(`^[a-zA-Z0-9_]{3,16}$`)

// OfflineUUIDNamespace mirrors what an offline-mode Minecraft server does
// when generating a UUID for a connecting unauthenticated client.
const OfflineUUIDNamespace = "OfflinePlayer:"

// LoginOffline produces an offline Profile for the given nick. The UUID is
// deterministic so the same nick always yields the same profile across
// installations, which means single-player worlds and offline-mode servers
// recognise the player on subsequent runs.
func LoginOffline(nick string) (Profile, error) {
	if !nickPattern.MatchString(nick) {
		return Profile{}, errors.New("invalid nick: 3-16 chars, [a-zA-Z0-9_] only")
	}
	id := offlineUUID(nick)
	return Profile{
		Type: TypeOffline,
		ID:   id.String(),
		Name: nick,
	}, nil
}

// offlineUUID computes the UUIDv3 of "OfflinePlayer:<nick>".
//
// We use uuid.Nil as the namespace, matching the upstream Minecraft Java
// implementation: java.util.UUID.nameUUIDFromBytes(("OfflinePlayer:" + nick).getBytes(UTF_8))
// is a UUIDv3 with no namespace prefix. uuid.NewMD5 gives the same result
// when called with uuid.Nil.
func offlineUUID(nick string) uuid.UUID {
	return uuid.NewMD5(uuid.Nil, []byte(OfflineUUIDNamespace+nick))
}
