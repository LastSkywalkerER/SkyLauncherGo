// Package httpclient is the launcher-wide HTTP client with retry, redirect,
// progress reporting and SHA-256 verification helpers.
package httpclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/progress"
)

const (
	defaultUserAgent = "SkyLauncher/0.0.0 (+https://github.com/LastSkywalkerER/SkyLauncherGo)"
	defaultTimeout   = 90 * time.Second
)

// Client wraps net/http with retry and progress reporting.
type Client struct {
	HTTP        *http.Client
	UserAgent   string
	MaxAttempts int
}

func New() *Client {
	return &Client{
		HTTP:        &http.Client{Timeout: defaultTimeout},
		UserAgent:   defaultUserAgent,
		MaxAttempts: 4,
	}
}

// Do issues the request with exponential-backoff retry on transient errors.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	req = req.WithContext(ctx)

	var lastErr error
	for attempt := 1; attempt <= c.MaxAttempts; attempt++ {
		resp, err := c.HTTP.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}
		if resp != nil {
			resp.Body.Close()
			lastErr = fmt.Errorf("%s %s: %s", req.Method, req.URL, resp.Status)
		} else {
			lastErr = err
		}
		if attempt == c.MaxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(1<<attempt) * time.Second):
		}
	}
	return nil, lastErr
}

// GetJSON performs a GET and decodes the JSON body into out.
func (c *Client) GetJSON(ctx context.Context, url string, headers map[string]string, out any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return decodeJSON(resp.Body, out)
}

// DownloadOptions configures a verified download.
type DownloadOptions struct {
	URL      string
	DestPath string
	SHA1     string // optional integrity check (Mojang manifests use SHA-1)
	SHA256   string // optional integrity check
	Size     int64  // optional expected byte size
	Headers  map[string]string
	Reporter progress.Reporter
	TaskID   string
	Stage    string
}

// Download fetches a URL into DestPath atomically, optionally verifying the
// hash and reporting progress. The destination directory is created on demand.
func (c *Client) Download(ctx context.Context, opts DownloadOptions) error {
	if opts.URL == "" || opts.DestPath == "" {
		return errors.New("download: URL and DestPath required")
	}
	if err := os.MkdirAll(filepath.Dir(opts.DestPath), 0o755); err != nil {
		return err
	}
	if opts.Reporter == nil {
		opts.Reporter = progress.Noop{}
	}

	req, err := http.NewRequest(http.MethodGet, opts.URL, nil)
	if err != nil {
		return err
	}
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	resp, err := c.Do(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("download %s: %s", opts.URL, resp.Status)
	}

	total := resp.ContentLength
	if opts.Size > 0 {
		total = opts.Size
	}

	tmp := opts.DestPath + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	hashers := []io.Writer{out}
	var sha1Sum, sha256Sum *hashAccumulator
	if opts.SHA1 != "" {
		sha1Sum = newHash("sha1")
		hashers = append(hashers, sha1Sum)
	}
	if opts.SHA256 != "" {
		sha256Sum = newHash("sha256")
		hashers = append(hashers, sha256Sum)
	}
	mw := io.MultiWriter(hashers...)

	pr := &progressReader{
		r:        resp.Body,
		total:    total,
		reporter: opts.Reporter,
		taskID:   opts.TaskID,
		stage:    orDefault(opts.Stage, progress.StageDownload),
		message:  filepath.Base(opts.DestPath),
	}
	_, copyErr := io.Copy(mw, pr)
	closeErr := out.Close()
	if copyErr != nil {
		os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		os.Remove(tmp)
		return closeErr
	}

	if sha1Sum != nil {
		if got := sha1Sum.Hex(); got != opts.SHA1 {
			os.Remove(tmp)
			return fmt.Errorf("sha1 mismatch for %s: want %s got %s", opts.URL, opts.SHA1, got)
		}
	}
	if sha256Sum != nil {
		if got := sha256Sum.Hex(); got != opts.SHA256 {
			os.Remove(tmp)
			return fmt.Errorf("sha256 mismatch for %s: want %s got %s", opts.URL, opts.SHA256, got)
		}
	}

	if err := os.Rename(tmp, opts.DestPath); err != nil {
		return err
	}
	opts.Reporter.Report(progress.Event{
		ID:    opts.TaskID,
		Stage: orDefault(opts.Stage, progress.StageDownload),
		Bytes: pr.read, TotalBytes: total,
		Done: true, Message: filepath.Base(opts.DestPath),
	})
	return nil
}

// ChecksumFile returns the SHA-256 hex digest of an existing file.
func ChecksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

type progressReader struct {
	r        io.Reader
	total    int64
	read     int64
	last     time.Time
	reporter progress.Reporter
	taskID   string
	stage    string
	message  string
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.read += int64(n)
		now := time.Now()
		if now.Sub(p.last) > 100*time.Millisecond {
			p.last = now
			p.reporter.Report(progress.Event{
				ID: p.taskID, Stage: p.stage, Message: p.message,
				Bytes: p.read, TotalBytes: p.total,
			})
		}
	}
	return n, err
}
