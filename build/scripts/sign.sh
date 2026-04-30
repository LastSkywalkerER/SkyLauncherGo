#!/usr/bin/env bash
# Compose latest.json by signing every artifact under build/bin/.
#
# Inputs (env):
#   UPDATER_PRIVATE_KEY  PEM-encoded ed25519 private key (single line, $'\n' newlines)
#   VERSION              Release semver (e.g. 1.2.3)
#   BASE_URL             Public URL prefix where the artifacts will be hosted
#                        (e.g. https://github.com/LastSkywalkerER/SkyLauncherGo/releases/download/v1.2.3)
#
# Output: ./latest.json next to the script's working directory.

set -euo pipefail

: "${UPDATER_PRIVATE_KEY:?missing UPDATER_PRIVATE_KEY}"
: "${VERSION:?missing VERSION}"
: "${BASE_URL:?missing BASE_URL}"

key_file=$(mktemp)
trap 'rm -f "$key_file"' EXIT
printf '%s' "$UPDATER_PRIVATE_KEY" > "$key_file"

files=()
for path in build/bin/SkyLauncher-*; do
  [[ -f "$path" ]] || continue
  fname=$(basename "$path")
  case "$fname" in
    *windows*)  platform=windows; arch=amd64 ;;
    *darwin*universal*)  platform=darwin; arch=universal ;;
    *darwin*amd64*)      platform=darwin; arch=amd64 ;;
    *darwin*arm64*)      platform=darwin; arch=arm64 ;;
    *linux*amd64*)       platform=linux;  arch=amd64 ;;
    *) echo "skipping unrecognised artifact $fname"; continue ;;
  esac
  sha=$(shasum -a 256 "$path" | awk '{print $1}')
  sig=$(printf '%s' "$sha" | openssl pkeyutl -sign -inkey "$key_file" -rawin -keyform PEM | base64 | tr -d '\n')
  size=$(stat -c '%s' "$path" 2>/dev/null || stat -f '%z' "$path")
  files+=("$(jq -n --arg p "$platform" --arg a "$arch" --arg u "$BASE_URL/$fname" --arg s "$sha" --arg sig "$sig" --argjson sz "$size" \
    '{platform:$p, arch:$a, url:$u, sha256:$s, signature:$sig, size:$sz}')")
done

printf '%s\n' "${files[@]}" |
  jq -s --arg v "$VERSION" --arg d "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
    '{version:$v, releaseDate:$d, notes:"", files:.}' \
    > latest.json
