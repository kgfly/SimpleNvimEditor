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

## License

MIT — see [LICENSE](LICENSE).
