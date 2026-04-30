# SkyLauncher

Cross-platform Minecraft launcher built on Go + Wails v3.

## Status

Active migration from Electron/NestJS (https://github.com/LastSkywalkerER/SkyLauncher) to Go + Wails v3.

## Stack

- Go 1.23+
- Wails v3 (alpha)
- React 18 + Vite + PrimeReact + TailwindCSS
- CurseForge API integration
- Microsoft (online) and offline-nick auth
- Vanilla, Fabric, Forge and NeoForge modloaders

## Build

Requires Go 1.23+, Node 22+, and Wails v3 CLI:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

Then:

```sh
wails3 dev      # development
wails3 task build   # production build
```

Set `CURSEFORGE_API_KEY` in your environment for development. CI builds inject the
key via `-ldflags`.

## Platforms

MVP: Windows x64, macOS Universal (arm64+amd64). Linux planned post-MVP.
