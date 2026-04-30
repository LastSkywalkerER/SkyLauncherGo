// Package launch builds the JVM argv for a resolved Minecraft version
// and starts the game process.
package launch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/xmcl/version"
)

// Profile holds the player identity passed to MC as command-line args.
type Profile struct {
	Username    string // visible name
	UUID        string // canonical UUID (with or without dashes — we normalise)
	AccessToken string // empty for offline (we substitute "0")
	UserType    string // "msa" | "legacy"
}

// Options bundles everything launch() needs that isn't on the resolved version.
type Options struct {
	Folder     string  // .minecraft root
	JavaPath   string  // absolute path to java(.exe)
	GameDir    string  // optional override; defaults to Folder
	HeapMB     int     // heap size; 0 = leave default
	JVMExtra   []string // user-supplied -X*, --add-opens, etc.
	Profile    Profile
	LauncherID string // exposed to MC as ${launcher_name}
	WindowW    int
	WindowH    int
}

// Build constructs the full argv slice.
func Build(resolved *version.Resolved, opts Options) ([]string, error) {
	if resolved == nil {
		return nil, fmt.Errorf("nil resolved version")
	}
	if opts.JavaPath == "" {
		return nil, fmt.Errorf("java path required")
	}
	if opts.GameDir == "" {
		opts.GameDir = opts.Folder
	}
	if opts.LauncherID == "" {
		opts.LauncherID = "skylauncher"
	}

	subs := substitutions(resolved, opts)

	jvm := jvmArgs(resolved, opts, subs)
	game := gameArgs(resolved, opts, subs)

	argv := make([]string, 0, len(jvm)+len(game)+2)
	argv = append(argv, opts.JavaPath)
	argv = append(argv, jvm...)
	argv = append(argv, resolved.MainClass)
	argv = append(argv, game...)
	return argv, nil
}

// Run launches the game. Stdout/stderr are inherited from the launcher
// process. Returns the *exec.Cmd so callers can track / kill it.
func Run(ctx context.Context, resolved *version.Resolved, opts Options) (*exec.Cmd, error) {
	argv, err := Build(resolved, opts)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = opts.GameDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// substitutions builds the placeholder map used by both jvm and game arg
// expansion. The keys mirror Mojang's documented ${...} variables.
func substitutions(r *version.Resolved, o Options) map[string]string {
	libsDir := filepath.Join(o.Folder, "libraries")
	nativesDir := filepath.Join(o.Folder, "versions", r.ID, "natives")
	classpath := buildClasspath(r, libsDir, o.Folder)
	access := o.Profile.AccessToken
	if access == "" {
		access = "0"
	}
	uuid := o.Profile.UUID
	if uuid == "" {
		uuid = "00000000-0000-0000-0000-000000000000"
	}
	return map[string]string{
		"auth_player_name":  orDefault(o.Profile.Username, "Player"),
		"auth_uuid":         strings.ReplaceAll(uuid, "-", ""),
		"auth_access_token": access,
		"auth_session":      access,
		"clientid":          "SkyLauncher",
		"auth_xuid":         "0",
		"user_type":         orDefault(o.Profile.UserType, "msa"),
		"user_properties":   "{}",
		"version_name":      r.ID,
		"version_type":      orDefault(r.Type, "release"),
		"game_directory":    o.GameDir,
		"assets_root":       filepath.Join(o.Folder, "assets"),
		"assets_index_name": orDefault(r.AssetIndex.ID, r.Assets),
		"game_assets":       filepath.Join(o.Folder, "assets", "virtual", "legacy"),
		"natives_directory": nativesDir,
		"launcher_name":     o.LauncherID,
		"launcher_version":  "0.0.0",
		"classpath":         classpath,
	}
}

func jvmArgs(r *version.Resolved, o Options, subs map[string]string) []string {
	var args []string
	if o.HeapMB > 0 {
		args = append(args, fmt.Sprintf("-Xmx%dM", o.HeapMB), fmt.Sprintf("-Xms%dM", o.HeapMB/2))
	}
	args = append(args, o.JVMExtra...)

	if len(r.JVMArguments) > 0 {
		for _, raw := range r.JVMArguments {
			args = append(args, expandArg(raw, subs)...)
		}
	} else {
		// Pre-1.13 versions don't ship JVM args; provide the defaults.
		args = append(args,
			"-Djava.library.path="+subs["natives_directory"],
			"-Dminecraft.launcher.brand="+subs["launcher_name"],
			"-Dminecraft.launcher.version="+subs["launcher_version"],
			"-cp", subs["classpath"],
		)
		if runtime.GOOS == "darwin" {
			args = append(args, "-XstartOnFirstThread")
		}
	}
	return args
}

func gameArgs(r *version.Resolved, o Options, subs map[string]string) []string {
	if r.MinecraftArguments != "" {
		fields := strings.Fields(r.MinecraftArguments)
		for i, f := range fields {
			fields[i] = expandPlaceholders(f, subs)
		}
		if o.WindowW > 0 && o.WindowH > 0 {
			fields = append(fields, "--width", fmt.Sprint(o.WindowW), "--height", fmt.Sprint(o.WindowH))
		}
		return fields
	}
	var args []string
	for _, raw := range r.GameArguments {
		args = append(args, expandArg(raw, subs)...)
	}
	if o.WindowW > 0 && o.WindowH > 0 {
		args = append(args, "--width", fmt.Sprint(o.WindowW), "--height", fmt.Sprint(o.WindowH))
	}
	return args
}

// expandArg handles one entry from arguments.game / arguments.jvm. The entry
// is either a JSON string or an object {rules:[...], value:string|[]string}.
func expandArg(raw json.RawMessage, subs map[string]string) []string {
	trim := strings.TrimSpace(string(raw))
	if len(trim) > 0 && trim[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil
		}
		return []string{expandPlaceholders(s, subs)}
	}
	var obj struct {
		Rules []version.Rule  `json:"rules"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	if !version.MatchRules(obj.Rules, nil) {
		return nil
	}
	val := strings.TrimSpace(string(obj.Value))
	if len(val) == 0 {
		return nil
	}
	if val[0] == '"' {
		var s string
		if err := json.Unmarshal(obj.Value, &s); err != nil {
			return nil
		}
		return []string{expandPlaceholders(s, subs)}
	}
	var arr []string
	if err := json.Unmarshal(obj.Value, &arr); err != nil {
		return nil
	}
	for i, s := range arr {
		arr[i] = expandPlaceholders(s, subs)
	}
	return arr
}

func expandPlaceholders(s string, subs map[string]string) string {
	for k, v := range subs {
		s = strings.ReplaceAll(s, "${"+k+"}", v)
	}
	return s
}

func buildClasspath(r *version.Resolved, libsDir, folder string) string {
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}
	var parts []string
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		parts = append(parts, p)
	}
	for _, l := range r.Libraries {
		if !version.MatchRules(l.Rules, nil) {
			continue
		}
		// Skip pure-native entries; they're extracted, not on cp.
		if l.Downloads != nil && l.Downloads.Artifact == nil && len(l.Downloads.Classifiers) > 0 {
			if version.LibraryNativeClassifier(l) != "" {
				continue
			}
		}
		var path string
		if l.Downloads != nil && l.Downloads.Artifact != nil {
			path = filepath.Join(libsDir, filepath.FromSlash(l.Downloads.Artifact.Path))
		} else if l.Name != "" {
			path = filepath.Join(libsDir, filepath.FromSlash(version.MavenPath(l.Name)))
		}
		add(path)
	}
	add(filepath.Join(folder, "versions", r.ID, r.ID+".jar"))
	return strings.Join(parts, sep)
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
