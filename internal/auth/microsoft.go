package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/httpclient"
)

// MicrosoftConfig holds the Azure AD app registration parameters.
//
// ClientID is injected at build time via -ldflags so a single binary can
// be branded per-distribution without rebuilding the source tree.
type MicrosoftConfig struct {
	ClientID    string
	Authority   string
	RedirectURI string
	Scopes      []string
}

// DefaultMicrosoftConfig matches the values used by the Electron-era
// SkyLauncher (legacy MSAL setup). The webview redirect URI is the special
// MS-provided desktop endpoint that doesn't require a local listener.
func DefaultMicrosoftConfig(clientID string) MicrosoftConfig {
	return MicrosoftConfig{
		ClientID:    clientID,
		Authority:   "https://login.microsoftonline.com/consumers",
		RedirectURI: "https://login.live.com/oauth20_desktop.srf",
		Scopes:      []string{"XboxLive.signin", "offline_access"},
	}
}

// AuthorizeURL builds the URL to load in the embedded webview. The browser
// will navigate through Microsoft's UI and eventually redirect to
// RedirectURI with a `code=...` query parameter.
func (c MicrosoftConfig) AuthorizeURL() string {
	u, _ := url.Parse(c.Authority + "/oauth2/v2.0/authorize")
	q := u.Query()
	q.Set("client_id", c.ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", c.RedirectURI)
	q.Set("response_mode", "query")
	q.Set("scope", strings.Join(c.Scopes, " "))
	q.Set("prompt", "select_account")
	u.RawQuery = q.Encode()
	return u.String()
}

// IsRedirect reports whether the given URL is the OAuth callback we should
// intercept and parse.
func (c MicrosoftConfig) IsRedirect(rawURL string) bool {
	return strings.HasPrefix(rawURL, c.RedirectURI+"?") || strings.HasPrefix(rawURL, c.RedirectURI+"#")
}

// ParseRedirect extracts the authorization code (or error) from a callback URL.
func ParseRedirect(rawURL string) (code string, err error) {
	u, perr := url.Parse(rawURL)
	if perr != nil {
		return "", perr
	}
	q := u.Query()
	if errParam := q.Get("error"); errParam != "" {
		return "", fmt.Errorf("microsoft auth: %s — %s", errParam, q.Get("error_description"))
	}
	code = q.Get("code")
	if code == "" {
		return "", errors.New("microsoft auth: no code in redirect")
	}
	return code, nil
}

// MicrosoftClient wraps the OAuth2 + Xbox + Minecraft auth chain.
type MicrosoftClient struct {
	HTTP *httpclient.Client
	Cfg  MicrosoftConfig
}

func NewMicrosoftClient(http *httpclient.Client, cfg MicrosoftConfig) *MicrosoftClient {
	return &MicrosoftClient{HTTP: http, Cfg: cfg}
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// ExchangeCode swaps an auth code for an MS access + refresh token.
func (c *MicrosoftClient) ExchangeCode(ctx context.Context, code string) (*tokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", c.Cfg.ClientID)
	form.Set("scope", strings.Join(c.Cfg.Scopes, " "))
	form.Set("code", code)
	form.Set("redirect_uri", c.Cfg.RedirectURI)
	form.Set("grant_type", "authorization_code")
	return c.tokenRequest(ctx, form)
}

// RefreshTokens exchanges a refresh token for fresh credentials.
func (c *MicrosoftClient) RefreshTokens(ctx context.Context, refreshToken string) (*tokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", c.Cfg.ClientID)
	form.Set("scope", strings.Join(c.Cfg.Scopes, " "))
	form.Set("refresh_token", refreshToken)
	form.Set("redirect_uri", c.Cfg.RedirectURI)
	form.Set("grant_type", "refresh_token")
	return c.tokenRequest(ctx, form)
}

func (c *MicrosoftClient) tokenRequest(ctx context.Context, form url.Values) (*tokenResponse, error) {
	req, err := http.NewRequest(http.MethodPost, c.Cfg.Authority+"/oauth2/v2.0/token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTP.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ms token: %s", resp.Status)
	}
	var t tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

// xboxLiveResponse is the auth response from user.auth.xboxlive.com.
type xboxLiveResponse struct {
	IssueInstant  time.Time `json:"IssueInstant"`
	NotAfter      time.Time `json:"NotAfter"`
	Token         string    `json:"Token"`
	DisplayClaims struct {
		Xui []struct {
			UHS string `json:"uhs"`
		} `json:"xui"`
	} `json:"DisplayClaims"`
}

// XboxLiveLogin trades an MS access token for an Xbox Live token.
func (c *MicrosoftClient) XboxLiveLogin(ctx context.Context, msAccessToken string) (*xboxLiveResponse, error) {
	body := map[string]any{
		"Properties": map[string]string{
			"AuthMethod": "RPS",
			"SiteName":   "user.auth.xboxlive.com",
			"RpsTicket":  "d=" + msAccessToken,
		},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	}
	var out xboxLiveResponse
	if err := c.postJSON(ctx, "https://user.auth.xboxlive.com/user/authenticate", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// XSTSLogin produces the XSTS token used to log into Minecraft Services.
// The returned response carries the same shape as XboxLiveLogin.
func (c *MicrosoftClient) XSTSLogin(ctx context.Context, xboxToken string) (*xboxLiveResponse, error) {
	body := map[string]any{
		"Properties": map[string]any{
			"SandboxId":  "RETAIL",
			"UserTokens": []string{xboxToken},
		},
		"RelyingParty": "rp://api.minecraftservices.com/",
		"TokenType":    "JWT",
	}
	var out xboxLiveResponse
	if err := c.postJSON(ctx, "https://xsts.auth.xboxlive.com/xsts/authorize", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type minecraftLoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// MinecraftLogin obtains the final Minecraft Services bearer token.
func (c *MicrosoftClient) MinecraftLogin(ctx context.Context, uhs, xstsToken string) (*minecraftLoginResponse, error) {
	body := map[string]any{
		"identityToken": "XBL3.0 x=" + uhs + ";" + xstsToken,
	}
	var out minecraftLoginResponse
	if err := c.postJSON(ctx, "https://api.minecraftservices.com/authentication/login_with_xbox", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MinecraftProfile is the player's MC identity.
type MinecraftProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FetchProfile loads the player's MC profile from Minecraft Services.
func (c *MicrosoftClient) FetchProfile(ctx context.Context, mcAccessToken string) (*MinecraftProfile, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.minecraftservices.com/minecraft/profile", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+mcAccessToken)
	resp, err := c.HTTP.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("microsoft account has no Minecraft profile")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("mc profile: %s", resp.Status)
	}
	var p MinecraftProfile
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

// CompleteFromCode runs the full chain: code → MS tokens → Xbox → XSTS →
// Minecraft → profile. It returns a fully-populated Profile ready for
// passing to launch.Run.
func (c *MicrosoftClient) CompleteFromCode(ctx context.Context, code string) (Profile, error) {
	tok, err := c.ExchangeCode(ctx, code)
	if err != nil {
		return Profile{}, fmt.Errorf("exchange code: %w", err)
	}
	xbl, err := c.XboxLiveLogin(ctx, tok.AccessToken)
	if err != nil {
		return Profile{}, fmt.Errorf("xbox live: %w", err)
	}
	if len(xbl.DisplayClaims.Xui) == 0 {
		return Profile{}, errors.New("xbox: missing user hash")
	}
	uhs := xbl.DisplayClaims.Xui[0].UHS
	xsts, err := c.XSTSLogin(ctx, xbl.Token)
	if err != nil {
		return Profile{}, fmt.Errorf("xsts: %w", err)
	}
	mc, err := c.MinecraftLogin(ctx, uhs, xsts.Token)
	if err != nil {
		return Profile{}, fmt.Errorf("minecraft login: %w", err)
	}
	prof, err := c.FetchProfile(ctx, mc.AccessToken)
	if err != nil {
		return Profile{}, err
	}
	return Profile{
		Type:         TypeMicrosoft,
		ID:           insertDashes(prof.ID),
		Name:         prof.Name,
		AccessToken:  mc.AccessToken,
		RefreshToken: tok.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(mc.ExpiresIn) * time.Second).Unix(),
	}, nil
}

func (c *MicrosoftClient) postJSON(ctx context.Context, url string, body, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(buf)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("POST %s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// insertDashes converts "abcdef..." (32 hex chars) into a canonical UUID
// "abcdef-...". The Minecraft Services API returns IDs without dashes.
func insertDashes(id string) string {
	if len(id) != 32 {
		return id
	}
	return id[0:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
}
