# Developer Guide

This is the practical "how do I build, run, and test this thing" doc.

SimpleNvimEditor (`simplenvim`) is a Go program that spawns a real `nvim` process
and renders it with [Gio](https://gioui.org), a GPU-accelerated, immediate-mode
GUI toolkit. Gio compiles to a normal Go binary on Linux, macOS, and Windows,
but — like any GUI toolkit that talks to the OS windowing system — it needs a
small amount of platform tooling to *compile* on each OS. This doc covers
that setup once per machine; after that, `go build` is all you need.

**Repo layout:** the Go module (all source, tests, `go.mod`, `go.sum`) lives
under [`src/`](../src). Docs live in `doc/` (this file, plus
[`ci-cd-setup.md`](ci-cd-setup.md) for how PR checks / nightly / release
builds are wired up). Every `go` command below assumes you've `cd`ed into
`src/` first.

## Prerequisites (all platforms)

- **Go 1.26+** — <https://go.dev/dl/> (the exact minimum is whatever
  `src/go.mod` declares; CI reads it from there rather than pinning a
  second copy of the version)
- **Neovim 0.9+** on your `PATH` (or pass `-nvim /path/to/nvim`) —
  <https://github.com/neovim/neovim/wiki/Installing-Neovim>

Everything else below is what your OS needs so that `cgo` can compile Gio's
windowing/GPU backend.

##  Linux

Gio's Linux backend uses `cgo` to talk to X11/Wayland and EGL/Vulkan, so you
need the corresponding `-dev` packages and `pkg-config` installed *before*
building. `go build` will fail with a `pkg-config ... was not found` error
listing exactly what's missing if you skip this.

**Debian / Ubuntu:**

```sh
sudo apt-get update
sudo apt-get install -y \
    pkg-config \
    libx11-dev libx11-xcb-dev libxkbcommon-dev libxkbcommon-x11-dev \
    libxcursor-dev libxfixes-dev \
    libwayland-dev \
    libgl1-mesa-dev libvulkan-dev
```

**Fedora:**

```sh
sudo dnf install -y pkg-config \
    libX11-devel libxkbcommon-devel libxkbcommon-x11-devel \
    libXcursor-devel libXfixes-devel \
    wayland-devel \
    mesa-libGL-devel vulkan-loader-devel
```

**Arch:**

```sh
sudo pacman -S --needed pkgconf libx11 libxkbcommon libxkbcommon-x11 \
    libxcursor libxfixes wayland mesa vulkan-icd-loader
```

You do **not** need a display server to *build*; you only need one to *run*
the GUI (a normal desktop session, or Wayland/X11 over SSH forwarding, or a
virtual display like Xvfb for headless testing).

## macOS

Gio's macOS backend uses Cocoa/Metal, which ship with the OS — you just need
the command-line build tools:

```sh
xcode-select --install
```

Install Neovim via Homebrew: `brew install neovim`.

## Windows

Gio's Windows backend uses Direct3D 11 via `cgo`, so you need a `gcc`
toolchain on `PATH`. The most common way to get one:

- Install [MSYS2](https://www.msys2.org/), then from an MSYS2 shell:
  ```sh
  pacman -S mingw-w64-x86_64-gcc
  ```
  and add `C:\msys64\mingw64\bin` to your Windows `PATH`.
- Or install [TDM-GCC](https://jmeubank.github.io/tdm-gcc/).

Install Neovim via `scoop install neovim` or the
[official installer](https://github.com/neovim/neovim/releases).

Verify `gcc` is visible to Go: `go env CGO_ENABLED` should print `1`, and
`gcc --version` should succeed in the same shell you run `go build` from.

## Build and run

All the Go code (and `go.mod`/`go.sum`) lives under [`src/`](../src), which
is the Go module root — `cd` there first:

```sh
cd src
go build ./...                            # sanity check everything compiles
go build -o simplenvim ./cmd/simplenvim   # produce a binary named simplenvim
                                           # (simplenvim.exe on Windows)
./simplenvim path/to/file.txt             # or just `./simplenvim` to open an
                                           # empty buffer
```

```sh
go build ./...; go build -o simplenvim ./cmd/simplenvim  # one line
```
or skip the explicit build and just:

```sh
cd src
go run ./cmd/simplenvim path/to/file.txt
```

### Command-line flags

| Flag | Description |
|---|---|
| `-nvim /path/to/nvim` | Overrides the `nvim` executable to launch (default: whatever the config file says, or plain `nvim` resolved via `PATH`). |

Any remaining positional arguments are treated as files to open, exactly
like `nvim file1 file2`.

For the `config.toml` file format (font/Nvim-launch settings, default
values, and file locations per OS), see the
[Configuration section in the README](../README.md#configuration).

## Testing

Tests live under `src/test/`, split into three tiers: `unit`, `integration`,
and `e2e`. Run everything at once from `src/` with:

```sh
go test ./...
```

### Unit tests — `test/unit`

No external dependencies (no Nvim, no display). These test the protocol
state machine (`uistate`), key/mouse translation (`input`), config loading
(`config`), and the Gio grid painter (`render`) in isolation, using
hand-built redraw batches instead of a real Nvim connection.

```sh
go test ./test/unit/...
go test ./test/unit/... -race         # check for data races (State is shared
                                       # between the redraw-pump goroutine and
                                       # the Gio frame goroutine)
go test ./test/unit/... -v            # see each test name
```

These always run and must always pass — they're the fast inner loop.

### Integration tests — `test/integration`

Spawn a real, isolated `nvim -u NONE -n --embed` child process (via the same
`nvimproc.Process` the app itself uses) and drive it through the actual
msgpack-rpc pipeline: attach, send key input, resize, send mouse events, and
quit — then verify the results both through our own mirrored `uistate` grid
*and* by asking the real Nvim process directly (e.g. `getline()`,
`&columns`), so a bug in our own decoding can't hide behind itself.

```sh
go test ./test/integration/...
```

These **skip themselves** (not fail) if `nvim` isn't found on `PATH`, so the
suite stays green on a machine without Neovim installed. On a machine that
has it, they spawn real processes and take a few seconds.

### End-to-end tests — `test/e2e`

Build the actual `simplenvim` binary, launch it under a virtual X11 display
(Xvfb), and verify it for real: a window with the right title appears, it
spawns an actual Nvim child process, a screenshot of the window shows
genuinely rendered content (not a blank frame), and killing the app cleans
up its Nvim child rather than leaking it.

```sh
go test ./test/e2e/...
```

These require, all on `PATH`: `nvim`, `go`, and (Linux-only) `Xvfb`,
`xdotool`, and ImageMagick's `import`. They **skip themselves** on
non-Linux, or if any tool is missing — Xvfb is X11-specific, so this tier
intentionally doesn't try to run on macOS/Windows dev machines.

On Debian/Ubuntu, the extra e2e-only tools are:

```sh
sudo apt-get install -y xvfb xdotool imagemagick
```

(`Xvfb` is provided by the `xvfb` package; ImageMagick provides `import`.)

### Coverage

Because the test tiers live in their own `_test` packages under `test/...`
rather than next to the source, plain `-cover` only reports on those
(mostly-empty) test packages themselves. To see real coverage of the
`internal/...` packages the tests exercise, use `-coverpkg`:

```sh
go test ./test/unit/... -coverpkg=./internal/... -coverprofile=cover.out
go tool cover -func=cover.out
```

## CI/CD

Workflows live in [`.github/workflows/`](../.github/workflows). The design
rationale (and the not-yet-implemented publishing channels) is in
[`ci-cd-setup.md`](ci-cd-setup.md).

| Workflow | Trigger | What it does |
|---|---|---|
| `pr.yml` | pull request / push to `main` | Build, vet, gofmt, unit + integration (`-race`) on Linux/macOS/Windows, e2e on Linux, coverage gate |
| `build-matrix.yml` | called by others | Reusable: builds the binary natively on all 6 OS/arch combos, uploads archives |
| `package.yml` | called by `release.yml` | Reusable: `.deb`/`.rpm` (nfpm), Windows `.exe` (Inno Setup), macOS `.dmg` (hdiutil) |
| `nightly.yml` | 09:00 UTC daily, or manual | Builds **only if `main` moved**; publishes `nightly-YYYYMMDD` pre-release, keeps the newest 7 |
| `release.yml` | tag `v*.*.*`, or manual | Full build + package + GitHub Release + build-provenance attestations |

### Coverage gate

`pr.yml` fails if total line coverage over `internal/...` drops below
**75%** (`MIN_COVERAGE` in the workflow). Coverage comes from the unit +
integration tiers only — e2e drives a separately-compiled binary, so its
execution isn't visible to `-coverpkg` (see §6).

The target in `ci-cd-setup.md` is 80%; the gate is set at 75% because
that's just under the measured 77.8% at the time it was added. Raise it as
the suite improves — it's a one-line change.

The gate, the gofmt check, and the e2e tier all run on Linux only. That's
deliberate: formatting is platform-independent, and the integration tier
*skips itself* when `nvim` is absent, so gating on a runner with a flaky
`nvim` install would fail for reasons unrelated to the change under test.

### Reproducing CI locally

```sh
cd src
go build ./... && go vet ./... && gofmt -l .
go test ./test/unit/... ./test/integration/... -race \
  -coverpkg=./internal/... -coverprofile=cover.out
go tool cover -func=cover.out | tail -1     # the gated number
go test ./test/e2e/...
```

Validate workflow edits before pushing — this catches bad expressions and
shell bugs that plain YAML linting misses:

```sh
actionlint    # https://github.com/rhysd/actionlint
```

### Triggering a nightly build manually

Go to **Actions → Nightly → Run workflow** on GitHub, optionally check
**"Build even if there are no new commits since the last nightly"**, and
click **Run workflow**. 

Or use the CLI:

```sh
gh workflow run nightly.yml                  # normal run (skips if no new commits)
gh workflow run nightly.yml -f force=true    # build regardless
```

### Releasing (official build)

Pushing to `main` does **not** create a release automatically. An official
release can be triggered in 3 ways:

Go to **Actions → Release → Run workflow**, enter the version (e.g.
`v1.0.0`), and click **Run workflow**. 

Or use the CLI:

```sh
gh workflow run release.yml -f version=v1.0.0
```

Or push a server tag

```sh
git tag v1.0.0
git push origin v1.0.0
```

Either way, `release.yml`:

1. Validates the version matches `vX.Y.Z`.
2. Builds the binary natively on all 6 OS/arch combinations
   (Linux/macOS/Windows × amd64/arm64) via `build-matrix.yml`.
   The version is stamped into the binary (`simplenvim --version`).
3. Produces native installers via `package.yml`:
   `.deb` and `.rpm` (nfpm), Windows `.exe` (Inno Setup),
   macOS `.dmg` (hdiutil).
4. Generates SHA-256 checksums and GitHub Artifact Attestations
   (supply-chain provenance).
5. Creates a GitHub Release with all assets attached.

Builds are **unsigned** (Phase 1): macOS needs `xattr -c`
or right-click → Open on first launch, Windows needs *More info* →
*Run anyway*. Signing/notarization is Phase 2 — see `ci-cd-setup.md` §8.

Publishing to winget, Homebrew, AUR, and apt/rpm repos is **not** wired
up, since each needs secrets and external accounts that don't exist yet.
`ci-cd-setup.md` §7 has the recipes for when that changes.

## Troubleshooting

**`pkg-config ... was not found in the pkg-config search path`** (Linux) —
install the missing `-dev` package(s) named in the error; see §2 above for
the full list up front.

**Build succeeds but the window never opens / immediately exits** — make
sure you actually have a display to render to (`$DISPLAY` or
`$WAYLAND_DISPLAY` set on Linux). For headless testing, run under Xvfb:
`Xvfb :99 -screen 0 1024x768x24 & DISPLAY=:99 ./simplenvim`.

**Nvim seems to open but nothing happens after that** — check that `nvim`
(or your configured `-nvim` path) actually starts standalone; a broken user
`init.lua`/`init.vim` that hangs on startup will hang `simplenvim` too, since
it's the same process.
