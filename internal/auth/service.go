package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/browser"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/httpclient"
)

// MicrosoftClientID is replaced at build time via:
//
//	go build -ldflags "-X 'github.com/LastSkywalkerER/SkyLauncherGo/internal/auth.MicrosoftClientID=<azure-app-id>'"
var MicrosoftClientID = ""

// Service is the Wails-bound facade exposing both auth flows to the UI.
type Service struct {
	HTTP *httpclient.Client

	mu      sync.Mutex
	current Profile
}

func NewService(http *httpclient.Client) *Service {
	return &Service{HTTP: http}
}

// Current returns the active profile, if any.
func (s *Service) Current() Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

// LoginOffline authenticates a user with a free-form nick.
func (s *Service) LoginOffline(nick string) (Profile, error) {
	p, err := LoginOffline(nick)
	if err != nil {
		return Profile{}, err
	}
	s.mu.Lock()
	s.current = p
	s.mu.Unlock()
	return p, nil
}

// Logout drops the cached profile. The frontend should call this before
// switching accounts.
func (s *Service) Logout() {
	s.mu.Lock()
	s.current = Profile{}
	s.mu.Unlock()
}

// LoginMicrosoft starts the Microsoft OAuth code flow using a transient
// loopback HTTP listener as the redirect target. The user's default
// browser is opened to the Microsoft login page; once they finish, the
// browser is redirected to http://127.0.0.1:<port>/callback?code=... and
// we capture the code.
//
// We deliberately use the loopback flow rather than an embedded webview
// because the Wails v3 alpha webview does not surface settled-URL events
// in a portable way across platforms.
func (s *Service) LoginMicrosoft(ctx context.Context) (Profile, error) {
	if MicrosoftClientID == "" {
		return Profile{}, errors.New("microsoft auth not configured: set MicrosoftClientID at build time")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Profile{}, fmt.Errorf("listen loopback: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	cfg := MicrosoftConfig{
		ClientID:    MicrosoftClientID,
		Authority:   "https://login.microsoftonline.com/consumers",
		RedirectURI: fmt.Sprintf("http://127.0.0.1:%d/callback", port),
		Scopes:      []string{"XboxLive.signin", "offline_access"},
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			errCh <- fmt.Errorf("microsoft auth: %s — %s", e, q.Get("error_description"))
			http.Error(w, "Sign-in failed; you can close this window.", http.StatusBadRequest)
			return
		}
		code := q.Get("code")
		if code == "" {
			errCh <- errors.New("microsoft auth: no code in callback")
			http.Error(w, "Missing code.", http.StatusBadRequest)
			return
		}
		codeCh <- code
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>SkyLauncher</title>
<body style="font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh;margin:0">
<div><h2>Sign-in complete</h2><p>You can close this window and return to SkyLauncher.</p></div></body>`))
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := browser.OpenURL(cfg.AuthorizeURL()); err != nil {
		return Profile{}, fmt.Errorf("open browser: %w", err)
	}

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return Profile{}, err
	case <-ctx.Done():
		return Profile{}, ctx.Err()
	case <-time.After(5 * time.Minute):
		return Profile{}, errors.New("microsoft sign-in timed out")
	}

	client := NewMicrosoftClient(s.HTTP, cfg)
	prof, err := client.CompleteFromCode(ctx, code)
	if err != nil {
		return Profile{}, err
	}
	s.mu.Lock()
	s.current = prof
	s.mu.Unlock()
	return prof, nil
}
