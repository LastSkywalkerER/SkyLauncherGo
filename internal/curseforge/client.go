// Package curseforge is a thin client for the public CurseForge REST API
// (api.curseforge.com). It provides modpack search/list/details and file
// metadata, plus a workaround for files whose authors disallowed third-party
// distribution (`allowModDistribution: false`).
//
// The API key is required by CurseForge. We bake it into the binary via
// build-time -ldflags so a single binary ships with the production key,
// and developers can override it via the CURSEFORGE_API_KEY env var.
package curseforge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/httpclient"
)

// APIBase is the canonical CurseForge v1 endpoint.
const APIBase = "https://api.curseforge.com/v1"

// MinecraftGameID is the constant CurseForge gameId for Minecraft.
const MinecraftGameID = 432

// ClassIDModpacks is the CurseForge classId for modpacks under MC.
const ClassIDModpacks = 4471

// APIKey is overwritten at build time:
//
//	go build -ldflags "-X 'github.com/LastSkywalkerER/SkyLauncherGo/internal/curseforge.APIKey=$KEY'"
//
// At runtime, an empty APIKey falls back to the CURSEFORGE_API_KEY env var.
var APIKey = ""

// Client is the CurseForge API surface used by the launcher.
type Client struct {
	HTTP *httpclient.Client
}

func NewClient(c *httpclient.Client) *Client {
	return &Client{HTTP: c}
}

func (c *Client) apiKey() string {
	if APIKey != "" {
		return APIKey
	}
	return os.Getenv("CURSEFORGE_API_KEY")
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	url := APIBase + path
	var rdr *strings.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = strings.NewReader(string(raw))
	}
	var bodyReader interface {
		Read(p []byte) (n int, err error)
	} = rdr
	if rdr == nil {
		bodyReader = nil
	}
	req, err := http.NewRequest(method, url, bodyReaderOrNil(bodyReader))
	if err != nil {
		return err
	}
	if rdr != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if k := c.apiKey(); k != "" {
		req.Header.Set("x-api-key", k)
	}
	resp, err := c.HTTP.Do(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("curseforge: missing or invalid x-api-key (got %s)", resp.Status)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("curseforge %s %s: %s", method, path, resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// bodyReaderOrNil exists so net/http.NewRequest sees a true nil body when
// we have no payload (passing a typed-nil io.Reader would set Content-Length
// to 0 and trigger TE: chunked behaviour we don't want for GETs).
func bodyReaderOrNil(r interface {
	Read(p []byte) (n int, err error)
}) (req interface {
	Read(p []byte) (n int, err error)
}) {
	if r == nil {
		return nil
	}
	return r
}

// SearchParams are the supported subset of /v1/mods/search query params.
type SearchParams struct {
	Search        string
	GameVersion   string
	CategoryID    int
	SortField     int    // 1=Featured 2=Popularity 3=LastUpdated 4=Name 5=Author 6=TotalDownloads
	SortOrder     string // "asc" | "desc"
	Index         int
	PageSize      int
	ClassID       int // omit to default to modpacks
}

// SearchModpacks returns paginated modpack results for the given query.
func (c *Client) SearchModpacks(ctx context.Context, p SearchParams) (*SearchResponse, error) {
	if p.PageSize == 0 {
		p.PageSize = 25
	}
	if p.ClassID == 0 {
		p.ClassID = ClassIDModpacks
	}
	q := []string{
		"gameId=" + strconv.Itoa(MinecraftGameID),
		"classId=" + strconv.Itoa(p.ClassID),
		"pageSize=" + strconv.Itoa(p.PageSize),
		"index=" + strconv.Itoa(p.Index),
	}
	if p.Search != "" {
		q = append(q, "searchFilter="+escape(p.Search))
	}
	if p.GameVersion != "" {
		q = append(q, "gameVersion="+escape(p.GameVersion))
	}
	if p.CategoryID > 0 {
		q = append(q, "categoryId="+strconv.Itoa(p.CategoryID))
	}
	if p.SortField > 0 {
		q = append(q, "sortField="+strconv.Itoa(p.SortField))
	}
	if p.SortOrder != "" {
		q = append(q, "sortOrder="+p.SortOrder)
	}
	var out SearchResponse
	if err := c.do(ctx, http.MethodGet, "/mods/search?"+strings.Join(q, "&"), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMod returns full details (incl. latestFiles, screenshots) for one mod.
func (c *Client) GetMod(ctx context.Context, modID int) (*Mod, error) {
	var out struct {
		Data Mod `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/mods/%d", modID), nil, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// ListModFiles returns file metadata for a mod, optionally filtered by MC version.
func (c *Client) ListModFiles(ctx context.Context, modID int, gameVersion string) ([]File, error) {
	path := fmt.Sprintf("/mods/%d/files", modID)
	if gameVersion != "" {
		path += "?gameVersion=" + escape(gameVersion)
	}
	var out struct {
		Data []File `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// GetFile returns a single file's metadata (URL, hashes, size).
func (c *Client) GetFile(ctx context.Context, modID, fileID int) (*File, error) {
	var out struct {
		Data File `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/mods/%d/files/%d", modID, fileID), nil, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// GetFiles batch-fetches file metadata. Used to resolve modpack manifest entries.
func (c *Client) GetFiles(ctx context.Context, fileIDs []int) ([]File, error) {
	if len(fileIDs) == 0 {
		return nil, nil
	}
	body := map[string][]int{"fileIds": fileIDs}
	var out struct {
		Data []File `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/mods/files", body, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// ResolveDownloadURL returns the best-effort URL to download a file even
// when the author opted out of third-party distribution. CurseForge's edge
// CDN follows a stable path scheme based on fileId, which we can construct
// when `downloadUrl` is null.
func ResolveDownloadURL(f File) string {
	if f.DownloadURL != "" {
		return f.DownloadURL
	}
	// Edge CDN trick: forgecdn.net splits the fileId into two segments —
	// the first 4 digits (or 1000-bucket) and the remainder.
	first := f.ID / 1000
	rest := f.ID % 1000
	return fmt.Sprintf("https://edge.forgecdn.net/files/%d/%d/%s", first, rest, f.FileName)
}

func escape(s string) string { return strings.ReplaceAll(s, " ", "%20") }

// Time is a wrapper around time.Time so we can be lenient about CF's date format.
type Time struct{ time.Time }

func (t *Time) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// CurseForge sometimes drops sub-second precision; try a fallback.
		parsed, err = time.Parse("2006-01-02T15:04:05", s)
		if err != nil {
			return err
		}
	}
	t.Time = parsed
	return nil
}
