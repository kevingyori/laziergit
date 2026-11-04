<img src="resources/icon.png" width="64" height="64" alt="">

# laziergit

[lazygit](https://github.com/jesseduffield/lazygit) as a desktop GUI, keeping what makes lazygit
fast (one-key actions, everything visible, your keybindings) while adding what a terminal can't do:
real type hierarchy, hairline borders, a full colour palette, a drawn commit graph, hover and mouse.

Built with [MyGo](https://mygo.egoist.dev)'s native UI (Go, GPU-drawn, no webview) on top of
lazygit's own git layer (`pkg/commands`), so git behaviour matches lazygit's.

```sh
go run . /path/to/repo
```

Uses your lazygit config and keybindings (`LG_CONFIG_FILE` or the standard config location).
Git credential prompts open in-app. Cmd shortcuts stay with macOS.

## Limitations

- Custom commands do not yet support terminal integration, suggestions, ANSI colour templates or all session properties.
- Remotes/tags: inspection, fetch and tag checkout only; use custom commands for management.
- Redo is not yet supported.

## Development

```sh
go test ./...                                   # headless UI tests drive real keypresses and git
SHOT_DIR=/tmp/shots go test ./internal/app -run TestShots   # screenshots for visual review
PERF_REPO=/path/to/big/repo go test ./internal/app -run TestPerf -v
LAZIERGIT_KEYS="4 <enter>" LAZIERGIT_CAPTURE=out.png go run . /path/to/repo   # capture the real window
```

## App icon

`assets/AppIcon.icon` is the layered Icon Composer source. Open it in Apple's
Icon Composer to edit the glass mascot and its separate expression layer.
`mygo.json` uses it for the macOS asset catalog; `go tool mygo build -skip-dmg`
compiles it with Xcode 26 or later and packages the catalog and legacy ICNS.

After editing, run `scripts/export-icon.sh` on macOS with full Xcode selected to
regenerate `resources/icon.png`, the README image and fallback for builds without
Icon Composer support. Keep the layer artwork free of baked-in lighting and masks;
Icon Composer supplies the material, shadow, and platform shape.
