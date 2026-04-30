package curseforge

// SearchResponse is the envelope for /v1/mods/search.
type SearchResponse struct {
	Data       []Mod      `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// Pagination matches CurseForge's standard pagination block.
type Pagination struct {
	Index       int `json:"index"`
	PageSize    int `json:"pageSize"`
	ResultCount int `json:"resultCount"`
	TotalCount  int `json:"totalCount"`
}

// Mod is the trimmed view of a CF "mod" entity (which includes modpacks).
type Mod struct {
	ID                   int          `json:"id"`
	GameID               int          `json:"gameId"`
	Name                 string       `json:"name"`
	Slug                 string       `json:"slug"`
	Summary              string       `json:"summary"`
	Status               int          `json:"status"`
	DownloadCount        int64        `json:"downloadCount"`
	IsFeatured           bool         `json:"isFeatured"`
	PrimaryCategoryID    int          `json:"primaryCategoryId"`
	ClassID              int          `json:"classId"`
	Authors              []Author     `json:"authors"`
	Logo                 *Logo        `json:"logo"`
	Screenshots          []Screenshot `json:"screenshots"`
	MainFileID           int          `json:"mainFileId"`
	LatestFiles          []File       `json:"latestFiles"`
	LatestFilesIndexes   []FileIndex  `json:"latestFilesIndexes"`
	Categories           []Category   `json:"categories"`
	DateCreated          Time         `json:"dateCreated"`
	DateModified         Time         `json:"dateModified"`
	DateReleased         Time         `json:"dateReleased"`
	AllowModDistribution *bool        `json:"allowModDistribution"`
	Links                Links        `json:"links"`
}

// Author is a CF profile credited on a mod.
type Author struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Logo references the icon image for a mod.
type Logo struct {
	ID           int    `json:"id"`
	ModID        int    `json:"modId"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ThumbnailURL string `json:"thumbnailUrl"`
	URL          string `json:"url"`
}

// Screenshot is one image hosted by CF for a mod listing.
type Screenshot struct {
	ID           int    `json:"id"`
	ModID        int    `json:"modId"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ThumbnailURL string `json:"thumbnailUrl"`
	URL          string `json:"url"`
}

// Category describes a CF category leaf.
type Category struct {
	ID                int    `json:"id"`
	GameID            int    `json:"gameId"`
	Name              string `json:"name"`
	Slug              string `json:"slug"`
	URL               string `json:"url"`
	IconURL           string `json:"iconUrl"`
	IsClass           bool   `json:"isClass"`
	ClassID           int    `json:"classId"`
	ParentCategoryID  int    `json:"parentCategoryId"`
	DisplayIndex      int    `json:"displayIndex"`
}

// File is CF's representation of one downloadable artifact (modpack zip,
// mod jar, server pack, ...).
type File struct {
	ID                  int          `json:"id"`
	GameID              int          `json:"gameId"`
	ModID               int          `json:"modId"`
	IsAvailable         bool         `json:"isAvailable"`
	DisplayName         string       `json:"displayName"`
	FileName            string       `json:"fileName"`
	ReleaseType         int          `json:"releaseType"` // 1=release 2=beta 3=alpha
	FileStatus          int          `json:"fileStatus"`
	Hashes              []Hash       `json:"hashes"`
	FileDate            Time         `json:"fileDate"`
	FileLength          int64        `json:"fileLength"`
	DownloadCount       int64        `json:"downloadCount"`
	DownloadURL         string       `json:"downloadUrl"` // null if author opted out
	GameVersions        []string     `json:"gameVersions"`
	SortableGameVersions []SortableGameVersion `json:"sortableGameVersions"`
	Dependencies        []FileDep    `json:"dependencies"`
	AlternateFileID     int          `json:"alternateFileId"`
	IsServerPack        bool         `json:"isServerPack"`
	ServerPackFileID    int          `json:"serverPackFileId"`
	Modules             []Module     `json:"modules"`
}

// Hash is a "value/algo" pair (1=SHA-1, 2=MD5).
type Hash struct {
	Value string `json:"value"`
	Algo  int    `json:"algo"`
}

// SortableGameVersion is the structured view of a "1.20.1" game version.
type SortableGameVersion struct {
	GameVersionName        string `json:"gameVersionName"`
	GameVersionPadded      string `json:"gameVersionPadded"`
	GameVersion            string `json:"gameVersion"`
	GameVersionReleaseDate Time   `json:"gameVersionReleaseDate"`
	GameVersionTypeID      int    `json:"gameVersionTypeId"`
}

// FileDep links a file to one of its dependencies (required, optional, ...).
type FileDep struct {
	ModID        int `json:"modId"`
	RelationType int `json:"relationType"` // 1=embedded 2=optional 3=required ...
}

// Module is CF's per-folder file listing inside a mod jar/zip (rarely needed).
type Module struct {
	Name        string `json:"name"`
	Fingerprint int64  `json:"fingerprint"`
}

// FileIndex is the trimmed entry inside latestFilesIndexes.
type FileIndex struct {
	GameVersion       string `json:"gameVersion"`
	FileID            int    `json:"fileId"`
	FileName          string `json:"filename"`
	ReleaseType       int    `json:"releaseType"`
	GameVersionTypeID int    `json:"gameVersionTypeId"`
	ModLoader         int    `json:"modLoader"` // 0=Any 1=Forge 4=Fabric 5=Quilt 6=NeoForge
}

// Links are external URLs shown in the CF UI.
type Links struct {
	WebsiteURL string `json:"websiteUrl"`
	WikiURL    string `json:"wikiUrl"`
	IssuesURL  string `json:"issuesUrl"`
	SourceURL  string `json:"sourceUrl"`
}

// Modpack manifest schema (the manifest.json found inside a CF modpack zip).
type ModpackManifest struct {
	Minecraft struct {
		Version    string             `json:"version"`
		ModLoaders []ModLoaderManifest `json:"modLoaders"`
	} `json:"minecraft"`
	ManifestType    string         `json:"manifestType"`
	ManifestVersion int            `json:"manifestVersion"`
	Name            string         `json:"name"`
	Version         string         `json:"version"`
	Author          string         `json:"author"`
	Files           []ModpackFile  `json:"files"`
	Overrides       string         `json:"overrides"`
}

// ModLoaderManifest names the modloader to install ("forge-47.2.0", etc.).
type ModLoaderManifest struct {
	ID      string `json:"id"`
	Primary bool   `json:"primary"`
}

// ModpackFile is one entry under "files" — a project + file id pair.
type ModpackFile struct {
	ProjectID int  `json:"projectID"`
	FileID    int  `json:"fileID"`
	Required  bool `json:"required"`
}
