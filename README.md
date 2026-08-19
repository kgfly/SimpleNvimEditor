# SimpleNvimEditor

A simple, fast, native Neovim GUI written in Go, using the
[Gio](https://gioui.org) UI toolkit — inspired by
[goneovim](https://github.com/akiyosi/goneovim), but written from
scratch (no code borrowed).

It drives your real, unmodified `nvim` — your config, your plugins, your
keymaps — inside a native, GPU-rendered window instead of a terminal
emulator. No Electron, no bundled browser engine, no reimplementation of
the editor: just Neovim, talking its standard UI protocol to a small Go
binary.

## What it offers

- **It's just Neovim.** A GUI shell around a real `nvim` process
  (`nvim_ui_attach`), not a fork or reimplementation of the editor.
- Native window rendering via Gio — no Electron, no embedded browser.
- Cross-platform: Linux, macOS, and Windows, on both x64 and arm64.
- Multigrid-aware rendering — window splits and floating windows (like
  completion popups) are drawn wherever Nvim actually places them.
- Cursor shape (block/beam/underline) synced live from Nvim's mode info.
- Keyboard and mouse input, including scroll wheel, mapped faithfully to
  Nvim's own input protocol.
- Live window resizing.
- A small, plain `config.toml` for font and Nvim-launch settings.

Simple and pure — no bloat, no bundled plugin marketplace, no telemetry.

## Installing

Grab a build for your platform from the
[Releases page](https://github.com/kgfly/SimpleNvimEditor/releases).
Linux gets `.deb`/`.rpm`/`.tar.gz`, Windows a `setup.exe`, macOS a `.dmg`,
for both x64 and arm64. Nightly pre-releases are built from `main`.

Neovim 0.9+ must be installed and on your `PATH` — this is a GUI *for*
Neovim, not a copy of it.

### First launch: unsigned builds

Releases are **not code-signed or notarized** (that needs a paid Apple
developer account and a Windows certificate). The binaries are fine; the OS
just can't verify who made them, so it warns once:

- **macOS** — right-click the app and choose *Open*, or:
  ```sh
  xattr -c /Applications/SimpleNvimEditor.app
  ```
- **Windows** — on the SmartScreen prompt, click *More info* → *Run anyway*.

Every release does carry
[GitHub build provenance](https://docs.github.com/actions/security-guides/using-artifact-attestations),
so you can cryptographically confirm a download really was built by this
repo's CI:

```sh
gh attestation verify <downloaded-file> -R kgfly/SimpleNvimEditor
```

## Configuration

SimpleNvimEditor reads an optional TOML config file from your OS's
standard config directory. It's entirely optional — sane defaults apply
if it's missing, or if any field is left out:

| OS | Path |
|---|---|
| Linux | `~/.config/simplenvimeditor/config.toml` |
| macOS | `~/Library/Application Support/simplenvimeditor/config.toml` |
| Windows | `%AppData%\simplenvimeditor\config.toml` |

```toml
[editor]
font_size = 14
use_system_fonts = false   # true = resolve font_family against installed system fonts
font_family = "monospace"  # only consulted when use_system_fonts = true

[nvim]
command = "nvim"           # path or PATH-resolved name
extra_args = []            # extra args passed straight to nvim
```

You don't need to create this file to get started — the defaults shown
above are exactly what's used if it's absent.

## Documentation

- [`doc/developer.md`](doc/developer.md) — everything about building,
  running, testing, and contributing.
- [`doc/ci-cd-setup.md`](doc/ci-cd-setup.md) — how CI/CD is wired up, and
  what's deliberately deferred.

## License

MIT — see [LICENSE](LICENSE).
