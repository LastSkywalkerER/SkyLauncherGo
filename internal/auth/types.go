// Package auth handles user identity for launching Minecraft.
//
// Two flows are supported:
//
//   - Microsoft (online): Azure AD OAuth2 authorization-code through an
//     embedded Wails webview, then the standard Xbox Live → XSTS →
//     Minecraft Services chain to obtain a profile and access token.
//
//   - Offline (cracked): the user picks a name; we deterministically derive
//     a UUIDv3 in the OfflinePlayer namespace, the same way an offline-mode
//     Minecraft server does. No network calls, no Mojang account.
package auth

const (
	TypeMicrosoft = "microsoft"
	TypeOffline   = "offline"
)

// Profile is the post-authentication identity used by the launcher.
type Profile struct {
	Type         string `json:"type"` // TypeMicrosoft | TypeOffline
	ID           string `json:"id"`   // canonical UUID with dashes
	Name         string `json:"name"`
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresAt    int64  `json:"expiresAt,omitempty"` // unix seconds
}
