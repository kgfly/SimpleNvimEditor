# SimpleNvimEditor

A simple, fast, native Neovim GUI written in [Go](https://go.dev/), using the
[Gio](https://gioui.org) UI toolkit for minimalist.

## Why another Nvim GUI???

- **Neovim-qt** is not actively maintained. I once filed a bug that took
  several months to get the maintainer's attention. Because the maintainer
  is busy with other projects, I ended up fixing it myself and submitting
  a pull request — which then took several more months to get merged.
- **goneovim** depends on a Qt binding that has been deprecated. Because
  of this, the maintainer has decided to abandon the project.
- **Neovide** has too many dependencies and is too heavy for my needs.
- Many other GUIs cannot run on all three platforms (macOS, Linux,
  Windows) or have other significant limitations.

...

A minimalist, I just want a simple, lightweight GUI that works on all my machines.
There are surprisingly few candidates that meet that bar.

## What it offers

- **It's just Neovim.** A GUI shell around a real `nvim` process
  (`nvim_ui_attach`), not a fork or reimplementation of the editor.
- Native window rendering via Gio — no Electron, no embedded browser.
- Cross-platform: Linux, macOS, and Windows.
- Multigrid-aware rendering — window splits and floating windows (like
  completion popups) are drawn wherever Nvim actually places them.
- Cursor shape (block/beam/underline) synced live from Nvim's mode info.
- Keyboard and mouse input, including scroll wheel, mapped faithfully to
  Nvim's own input protocol.
- Live window resizing.
- A small, plain `config.toml` for font and Nvim-launch settings.
- Nerd font support.
- GPU rendering from Go/Gio.
- Seamlessly serve as your favourite terminal, by "simplenvim --maximized -- -c term -c startinsert". So you don't need other terminal software on your box.

Simple and pure — no bloat, no bundled plugin marketplace, no telemetry.

## Installing

Grab a build for your platform from the
[Releases page](https://github.com/kgfly/SimpleNvimEditor/releases).
Nightly pre-releases are built from `main`.

Neovim 0.9+ must be installed and on your `PATH` — this is a GUI *for*
Neovim, not a copy of it.

### Linux: `.deb` / `.rpm`

Install the downloaded package with an explicit path (the leading `./`
matters — without it, apt looks for a *package named* `simplenvim_...`):

```sh
sudo apt install ./simplenvim_<version>_linux_amd64.deb   # Debian/Ubuntu
sudo dnf install ./simplenvim_<version>_linux_amd64.rpm   # Fedora/RHEL
```

If apt prints:

```
Notice: Download is performed unsandboxed as root as file '...' couldn't be
accessed by user '_apt'. - pkgAcquire::Run (13: Permission denied)
```

nothing is wrong with the package. Apt drops privileges to the unprivileged
`_apt` user to copy the file, and that user can't traverse a private home
directory such as a `0700` `~/Downloads`. Apt falls back to running as root
and the install still succeeds — it is a notice, not an error.

To silence it, install from a world-traversable directory instead:

```sh
cp simplenvim_<version>_linux_amd64.deb /tmp/
sudo apt install /tmp/simplenvim_<version>_linux_amd64.deb
```

Prefer this over loosening the permissions on your home directory.

### First launch: unsigned builds

Releases are **not code-signed** (that requires a paid Apple developer
account and a Windows certificate). The binaries are fine; the OS just
can't verify who made them, so it warns on first launch.

**All installation packages are built exclusively by the GitHub CI/CD
pipeline with all security checks enabled. No third party is involved in
the package creation.**

- **macOS** — right-click the app and choose *Open*, or:
  ```sh
  xattr -c /Applications/SimpleNvimEditor.app
  ```
- **Windows** — on the SmartScreen prompt, click *More info* → *Run anyway*.

Every release does carry
[GitHub build provenance](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations),
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
font_family = "monospace"  # set to your preferred font, e.g. "Hack Nerd Font Mono"
alt_is_meta = true         # send Alt/Option chords to Nvim as <A-...>

[nvim]
command = "nvim"           # path or PATH-resolved name
extra_args = []            # extra args passed straight to nvim
```

You don't need to create this file to get started — the defaults shown
above are exactly what's used if it's absent.

### `alt_is_meta`

Only meaningful on macOS, where Option is a composing key: Option-a types
`å` and Option-Shift-a types `Å`. Keeping the default `true` means those
chords go to Nvim so `<A-a>` and `<A-A>` mappings fire. Set it to `false`
if you would rather type composed characters than use Option as Meta.

On Linux and Windows, Alt is already a pure command modifier and never
produces text, so this setting has no effect there.

## Command-line arguments

Use `--maximized` to start with a maximized window. Arguments after `--` are
forwarded unchanged to Nvim, including commands and Nvim flags:

```sh
simplenvim --maximized -- -c term -c edit /Users/user1/todo.txt
```

## Reporting issues

If an issue of yours was closed but the problem isn't actually fixed, comment
`/reopen` on it. A bot reopens it automatically — no write access needed, and
you don't have to wait for a maintainer.

This only works on issues **you** opened, and `/reopen` must start the comment.

## Documentation

- [`doc/developer.md`](doc/developer.md) — everything about building,
  running, testing, and contributing.

## License

MIT — see [LICENSE](LICENSE).
