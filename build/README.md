# Build artefacts and scripts

This directory holds non-source build helpers.

- `scripts/sign.sh` — composes `latest.json` for the self-updater by
  signing every artefact in `build/bin/` with an ed25519 key. CI calls
  it after the matrix build step. See the script's header for required
  env vars.

The release CI workflow lives in `.github/workflows/release.yml`. It
builds Windows x64 and macOS universal binaries, signs them, and uploads
both the binaries and `latest.json` to the GitHub Release.

## Required GitHub secrets

| Secret | Used for |
|---|---|
| `CURSEFORGE_API_KEY` | Baked into the binary so search/install works without per-user setup. |
| `MICROSOFT_CLIENT_ID` | Azure AD app registration for MS sign-in. |
| `UPDATER_PUBLIC_KEY` | Base64 ed25519 public key, embedded for signature verification. |
| `UPDATER_PRIVATE_KEY` | PEM ed25519 private key, used by `sign.sh` only — never embedded. |
| `UPDATER_MANIFEST_URL` | Public URL of `latest.json`. |
