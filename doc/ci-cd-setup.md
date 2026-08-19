# CI/CD Setup Guide

This document describes how to wire up **free** CI (pull-request checks)
and CD (nightly + official release builds/packages) for SimpleNvimEditor
as a public, open-source GitHub repo. It's a planning/reference doc —
the actual `.github/workflows/*.yml` files described here haven't been
committed yet; this is the blueprint to implement them from.

**Hard constraint honored throughout this doc:** every single step below
runs on infrastructure hosted by GitHub or a free third-party community
service (GitHub Actions runners, GitHub Pages, GitHub Releases, the AUR,
a Homebrew tap repo, the `winget-pkgs` repo, etc.) — **nothing here ever
builds, signs, or hosts anything on your own local machine.** Where a
step says "publish to GitHub Pages," that means a free GitHub-hosted
static site, not anything you run yourself.

**Preference order used throughout this doc:** since the code already
lives on GitHub, **GitHub's own free resources are used wherever they
can do the job** — GitHub Actions (compute), GitHub Releases (binary
hosting), GitHub Pages (static repo hosting), GitHub Artifact
Attestations (supply-chain provenance, §8), and the fact that
`winget-pkgs` and a Homebrew tap are *themselves* just GitHub repos. A
non-GitHub free service (AUR, Launchpad, COPR, SignPath.io) is only
brought in for the handful of things GitHub genuinely has no offering
for at all — each of those spots is called out explicitly below so it's
clear *why* it isn't GitHub.

**TL;DR — is this all doable for free?** Almost entirely yes, with two
honest exceptions called out up front so there's no surprise later:

| Piece | Free? | GitHub-native? |
|---|---|---|
| PR CI (build/vet/test/coverage-gate on Linux+macOS+Windows) | Yes, fully free | Yes — GitHub Actions |
| Nightly build | Yes, fully free | Yes — GitHub Actions + GitHub Releases |
| Release builds for Linux + Windows (x64 **and** arm64) | Yes, fully free | Yes — GitHub Actions |
| Release builds for macOS (x64 **and** arm64) | Yes, fully free | Yes — GitHub Actions |
| `.deb` / `.rpm` / Arch packages (building them) | Yes, fully free | Yes — built inside GitHub Actions |
| Hosting a `.deb`/`.rpm`/Arch repo | Yes, fully free | Yes, primarily — GitHub Pages (AUR is the one non-GitHub piece, used as a discoverability add-on for Arch; see §7.3) |
| Windows installer (`.exe`) | Yes, fully free | Yes — built + hosted via GitHub Actions/Releases |
| macOS `.dmg` | Yes, fully free to *build* | Yes — built + hosted via GitHub Actions/Releases |
| Publishing to a `winget` repo | Yes, fully free | Yes — `winget-pkgs` is itself a GitHub repo; automated via a GitHub Action |
| Publishing to a Homebrew tap | Yes, fully free | Yes — a "tap" is just a GitHub repo you own |
| Supply-chain provenance ("this binary really came from this repo's CI") | Yes, fully free | Yes — GitHub Artifact Attestations, native, §8 |
| **GitHub Packages as an apt/rpm host** | **Not doable.** GitHub Packages only supports npm, RubyGems, Maven, Gradle, Docker/Container images, and NuGet — there is no apt/deb or yum/rpm registry type. Verified against GitHub's own docs (see §10). | N/A |
| **macOS Gatekeeper-clean signing (notarization)** | **Not doable for free.** Requires an Apple Developer Program membership (**$99/year** — verified on Apple's own site). Workaround below ships an unsigned, still-installable `.dmg`. | No — Apple-only, no GitHub equivalent exists |
| **Windows SmartScreen-clean signing** | Normally paid, but a legitimate free option exists for OSS projects (SignPath.io's open-source program) — see §8. Otherwise ship unsigned. | No — no GitHub equivalent; SignPath.io is a third-party free service |

Everything else in this doc is genuinely $0/month using GitHub itself for
public repositories (Actions usage is explicitly documented as free for
public repos on standard GitHub-hosted runners) plus, only where GitHub
has no offering of its own, free community infrastructure (AUR,
Launchpad, COPR). See §10 for exactly what was verified and how.

---

## Implementation phases

This whole doc splits cleanly into two phases:

- **Phase 1 — everything a free resource can do** (§1–§7, §9, §10, and
  the free parts of §8): PR CI, nightly/release builds for all 6 OS/arch
  combinations, `.deb`/`.rpm`/Arch packaging and hosting, unsigned
  Windows/macOS installers, winget, Homebrew, AUR, and GitHub Artifact
  Attestations. **This is the part to actually build first** — it's a
  complete, real, working pipeline on its own, and ships a fully
  functional (if unsigned) app on every platform.
- **Phase 2 — the items that are not free** (§8, the "Not doable for
  free" rows): macOS notarization and — if the free SignPath.io
  application isn't approved — Windows code signing. These only affect
  whether the OS shows a scary-but-dismissable warning before first
  launch; the software itself works identically either way. Treat this
  as "nice to have once the project has some traction and it feels
  worth the money/effort," not a blocker to shipping Phase 1.

---

## 1. One-time repo setup

1. **Make sure the repo is public.** GitHub-hosted standard runners are
   documented as free with no minute cap for public repos; private
   repos instead get a small monthly minute quota and then start
   charging (see §10).
2. **Enable Actions** (Settings → Actions → General → allow all actions).
3. Create the workflow files under `.github/workflows/` (see §3–§5 for
   their contents):
   - `pr.yml` — runs on every pull request
   - `nightly.yml` — scheduled build from the default branch
   - `release.yml` — runs when a version tag is pushed
4. **Turn on branch protection** (Settings → Branches → Add rule for
   `main`):
   - Require a pull request before merging.
   - **Require status checks to pass before merging**, and select the
     `pr.yml` job(s) once they've run at least once (they only show up
     in the picker after the workflow has executed once on a PR).
   - Optionally require branches to be up to date before merging.

   This last step is the part that actually *blocks* a failing PR — a
   workflow failing on its own doesn't stop a merge unless a branch
   protection rule says so.

---

## 2. Build matrix & hosted-runner reality check

The target matrix is 3 OSes × 2 architectures = 6 combinations. All the
runner labels below are listed by GitHub as **"Standard GitHub-hosted
runners for public repositories"** — genuinely free, not the
separately-priced "larger runners" tier (which costs money even on
public repos). This table reflects GitHub's own current reference docs
(verified this session — see §10):

| OS | x64 | arm64 |
|---|---|---|
| Linux | `ubuntu-latest` (native, standard/free) | `ubuntu-24.04-arm` (native, standard/free) |
| Windows | `windows-latest` (native, standard/free) | `windows-11-arm` (native, standard/free) |
| macOS | `macos-15-intel` (native, Intel, standard/free) | `macos-latest` (native, Apple Silicon — GitHub's default macOS image, standard/free) |

**Why native runners instead of cross-compiling:** this project uses
`cgo` (Gio needs X11/Wayland/EGL/Vulkan headers on Linux, Direct3D11 on
Windows, Cocoa/Metal on macOS — see `doc/developer.md` §2–4). Real
cross-compilation of cgo code needs a matching sysroot/toolchain per
target and is fragile for GUI libraries specifically. Building natively
on a matching runner sidesteps that entirely, and GitHub's standard
(free) runner fleet now covers all 6 combinations natively — no need to
pay for, or self-host, arm64 hardware.

> **If your account doesn't have an arm64 runner label available** (some
> labels are marked "public preview" and availability can vary by
> account — check Settings → Actions → Runners, or just try the label
> and see if a job picks one up): fall back to emulated builds via
> [`docker/setup-qemu-action`](https://github.com/docker/setup-qemu-action)
> running an `arm64` Ubuntu container for Linux arm64, and consider
> dropping Windows arm64 from the matrix until the label is generally
> available rather than fighting cross-compilation — it's a small
> enough user base that "coming soon" is a reasonable interim answer.

**macOS x64 note:** `macos-latest` is Apple Silicon (arm64). To also
produce an x64 macOS build, build separately on the `macos-15-intel`
label (a real Intel Mac, still free) rather than cross-compiling from
the arm64 runner — simpler, and avoids fighting Apple clang's
cross-target flags for a cgo/GUI binary.

**Free-plan concurrency, for planning purposes:** GitHub's Free plan
allows 20 total concurrent Actions jobs, of which at most 5 can be
macOS jobs at once (shared between standard and larger macOS runners).
A 6-way release matrix (2 of which are macOS) fits comfortably within
that — nothing here needs a paid plan to run in parallel.

---

## 3. PR CI — `pr.yml`

Runs on every pull request. Builds + vets + tests on all three OSes, and
fails the check (blocking merge, once branch protection is on) if any
test fails **or** if total line coverage drops below 80%.

```yaml
name: PR

on:
  pull_request:
    branches: [main]

jobs:
  test:
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    runs-on: ${{ matrix.os }}
    defaults:
      run:
        working-directory: src
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      # --- OS-specific build dependencies (see doc/developer.md) ---
      - name: Install Linux build deps
        if: runner.os == 'Linux'
        run: |
          sudo apt-get update
          sudo apt-get install -y \
            pkg-config libx11-dev libx11-xcb-dev libxkbcommon-dev \
            libxkbcommon-x11-dev libxcursor-dev libxfixes-dev \
            libwayland-dev libgl1-mesa-dev libvulkan-dev \
            xvfb xdotool imagemagick neovim

      - name: Install macOS build deps
        if: runner.os == 'macOS'
        run: brew install neovim

      - name: Install Windows build deps
        if: runner.os == 'Windows'
        run: |
          choco install neovim -y
          # mingw-w64 gcc, needed for cgo: ships on windows-latest via
          # MSYS2 already installed at C:\msys64. Just add it to PATH.
          echo "C:\msys64\mingw64\bin" | Out-File -FilePath $env:GITHUB_PATH -Encoding utf8 -Append

      - name: Build
        run: go build ./...

      - name: Vet
        run: go vet ./...

      - name: Gofmt check
        if: runner.os != 'Windows'
        run: |
          fmtout=$(gofmt -l .)
          if [ -n "$fmtout" ]; then
            echo "gofmt found unformatted files:"
            echo "$fmtout"
            exit 1
          fi

      - name: Unit + integration tests with coverage
        run: |
          go test ./test/unit/... ./test/integration/... -race \
            -coverpkg=./internal/... -coverprofile=cover.out

      - name: End-to-end tests (Linux only)
        if: runner.os == 'Linux'
        run: go test ./test/e2e/...

      - name: Enforce 80% line coverage
        shell: bash
        run: |
          pct=$(go tool cover -func=cover.out | tail -1 | grep -oE '[0-9]+\.[0-9]+')
          echo "Total line coverage: ${pct}%"
          awk -v p="$pct" 'BEGIN { exit !(p >= 80) }' \
            || { echo "Coverage ${pct}% is below the required 80%"; exit 1; }
```

Notes:

- Coverage is computed from **unit + integration** tests only (they
  instrument `internal/...` directly). The e2e tier builds and runs a
  separate compiled binary and isn't included in the same coverage
  profile — see `doc/developer.md` §6 for why the tiers are split this
  way. This is a normal, common convention (e2e verifies real
  end-user behavior, not code coverage).
- `fail-fast: false` so one OS failing doesn't cancel the others —
  useful for seeing all three results in one PR run.
- Instead of the hand-rolled coverage-gate script, you can swap in a
  ready-made action like
  [`vladopajic/go-test-coverage`](https://github.com/vladopajic/go-test-coverage)
  for nicer PR annotations/summaries — also free for public repos. The
  script above is included so the whole pipeline works with zero
  third-party trust/signup if you'd rather not add a marketplace action.

---

## 4. CD — Nightly build — `nightly.yml`

Goal: once a day, **only if there's been a new commit** since the last
nightly, build all 6 platform/arch combinations and publish them as a
rolling "nightly" pre-release on GitHub Releases.

```yaml
name: Nightly

on:
  schedule:
    - cron: '0 9 * * *'   # 09:00 UTC daily
  workflow_dispatch: {}    # allow manual trigger too

jobs:
  check-for-changes:
    runs-on: ubuntu-latest
    outputs:
      changed: ${{ steps.check.outputs.changed }}
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - id: check
        run: |
          # Compare current HEAD to whatever commit the "nightly" tag
          # currently points at (if it exists yet).
          if git rev-parse nightly >/dev/null 2>&1; then
            last=$(git rev-list -n 1 nightly)
            head=$(git rev-parse HEAD)
            if [ "$last" = "$head" ]; then
              echo "changed=false" >> "$GITHUB_OUTPUT"
              exit 0
            fi
          fi
          echo "changed=true" >> "$GITHUB_OUTPUT"

  build:
    needs: check-for-changes
    if: needs.check-for-changes.outputs.changed == 'true'
    uses: ./.github/workflows/build-matrix.yml   # shared reusable workflow, see §5

  publish:
    needs: [check-for-changes, build]
    if: needs.check-for-changes.outputs.changed == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          path: dist
      - uses: softprops/action-gh-release@v2
        with:
          tag_name: nightly
          name: Nightly build
          prerelease: true
          # Force-move the "nightly" tag to the new commit each run.
          target_commitish: ${{ github.sha }}
          files: dist/**/*
```

This keeps a single, always-current `nightly` GitHub Release/tag rather
than accumulating a new release object every day — cleaner for users who
just want "latest nightly," and avoids ever running (and paying Actions
minutes for) a build when nothing changed.

---

## 5. CD — Official release build — `release.yml`

Goal: pushing a version tag (`v1.2.3`) builds all 6 combinations,
produces installers/packages for each OS, attaches them to a GitHub
Release, and pushes updates to the winget/Homebrew/package-repo
channels described in §7.

```yaml
name: Release

on:
  push:
    tags: ['v*.*.*']

jobs:
  build:
    uses: ./.github/workflows/build-matrix.yml

  package:
    needs: build
    uses: ./.github/workflows/package.yml

  publish-github-release:
    needs: package
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          path: dist
      - uses: softprops/action-gh-release@v2
        with:
          files: dist/**/*
          generate_release_notes: true

  publish-winget:
    needs: publish-github-release
    uses: ./.github/workflows/publish-winget.yml
  publish-homebrew:
    needs: publish-github-release
    uses: ./.github/workflows/publish-homebrew.yml
  publish-aur:
    needs: publish-github-release
    uses: ./.github/workflows/publish-aur.yml
  publish-apt-rpm:
    needs: publish-github-release
    uses: ./.github/workflows/publish-apt-rpm.yml
```

The `build-matrix.yml` and `package.yml` files are reusable workflows
(`workflow_call`) shared between `nightly.yml` and `release.yml` so the
build logic is written once. The recommended tool to actually drive most
of this is **[GoReleaser](https://goreleaser.com/)** (free, open-source,
MIT-licensed) — it natively understands Go cross-builds, archives,
checksums, changelogs, GitHub Releases, `nfpm`-based `.deb`/`.rpm`
packaging, and Homebrew tap publishing, which covers a large chunk of
`package.yml` and `publish-homebrew.yml` in one config file
(`.goreleaser.yaml`) instead of hand-rolled shell steps.

A minimal `.goreleaser.yaml` sketch:

```yaml
version: 2
builds:
  - id: simplenvim
    main: ./cmd/simplenvim
    dir: src
    goos: [linux, windows, darwin]
    goarch: [amd64, arm64]
    env: [CGO_ENABLED=1]   # required — Gio needs cgo, see §2

nfpms:
  - id: simplenvim-linux
    package_name: simplenvim
    formats: [deb, rpm]
    maintainer: "Your Name <you@example.com>"
    homepage: "https://github.com/kgfly/SimpleNvimEditor"
    description: "A native Neovim GUI written in Go + Gio"
    license: MIT

archives:
  - id: default
    formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]

brews:
  - name: simplenvimeditor
    repository:
      owner: kgfly
      name: homebrew-tap   # your own tap repo, see §7.5
    homepage: "https://github.com/kgfly/SimpleNvimEditor"
    description: "A native Neovim GUI written in Go + Gio"
```

Run it in CI with `goreleaser release --clean` (needs `GITHUB_TOKEN`,
which Actions provides automatically for free). Note GoReleaser still
needs to run its `builds:` step **per OS**, since cgo builds aren't truly
cross-platform (§2) — the common pattern is running GoReleaser in
"build-only" mode (`goreleaser build --single-target`) on each OS runner
in the matrix, uploading each OS's binaries as artifacts, then running
one final `goreleaser release` job on Linux that downloads all the
artifacts and does the packaging/publishing/GitHub-Release steps. See
GoReleaser's ["Building and testing for multiple platforms"
docs](https://goreleaser.com/cookbooks/) for the exact split pattern —
this is the standard approach for any cgo-using Go project doing
multi-OS releases with GoReleaser.

`.deb`/`.rpm` via `nfpms:` covers Debian/Ubuntu and Fedora/RHEL
packaging. For Arch, `nfpm` has experimental Arch package support in
recent versions, but the standard, most-compatible route for Arch users
is publishing a `PKGBUILD` to the AUR instead (§7.3) — treat `nfpm`'s
Arch output as optional/secondary.

---

## 6. Producing the OS-specific installers

### Linux: `.deb` / `.rpm` / Arch

Covered by `nfpm` above for `.deb`/`.rpm`. For Arch, maintain a
`PKGBUILD` (a small shell-like recipe) in the repo and publish it to the
AUR — see §7.3.

### Windows: installer `.exe`

Use **[Inno Setup](https://jrsoftware.org/isinfo.php)** (free, and its
Wine build runs fine on `windows-latest` via `choco install innosetup`).
Write an `.iss` script describing the install (target dir, Start Menu
shortcut, PATH registration if desired) and compile it headlessly:

```powershell
choco install innosetup -y
iscc installer.iss
```

This produces a single `simplenvim-setup.exe`. (WiX Toolset + `.msi` is the
other common free option if an MSI is specifically wanted for enterprise
deployment scenarios — Inno Setup's `.exe` is simpler and is what most
small OSS Windows apps ship.)

### macOS: `.dmg`

No extra tooling needed — `hdiutil` ships with macOS:

```sh
mkdir dmg-root
cp -R simplenvim.app dmg-root/
ln -s /Applications dmg-root/Applications
hdiutil create -volname "SimpleNvimEditor" -srcfolder dmg-root \
  -ov -format UDZO simplenvim.dmg
```

(`create-dmg` is a nicer-looking free alternative if you want a custom
background/icon layout in the Finder window; `hdiutil` alone is enough
for a functional, no-cost `.dmg`.)

You'll first need a minimal `.app` bundle wrapping the `simplenvim` binary
(an `Info.plist` + the executable under `Contents/MacOS/`) — this part
is just file layout, no paid tooling required.

---

## 7. Registering with package-repo services

**On GitHub Packages, upfront:** it's tempting to assume GitHub's own
package registry can host these, but it explicitly cannot — GitHub
Packages only supports npm, RubyGems, Apache Maven, Gradle, Docker/
Container images, and NuGet (verified against GitHub's docs, §10).
**There is no apt/deb or yum/rpm registry type on GitHub Packages.**
The free path for those formats is hosting your own repo metadata as a
plain static site, which is what §7.1–7.2 below describe — emphasis on
"hosting a static site," not "running a server": GitHub Pages is a free
GitHub-hosted service, so this still satisfies "no local box."

### 7.1 APT (Debian/Ubuntu)

Two free routes, both entirely remote (no local hosting):

- **GitHub-Pages-hosted repo** (built inside a GitHub Actions job, then
  published to Pages — full control, zero cost, nothing runs on your
  machine): build repo metadata with [`aptly`](https://www.aptly.info/)
  or `reprepro`, sign it with a GPG key you generate and keep as a repo
  secret, and publish the resulting static files to a `gh-pages` branch
  (via the `actions/deploy-pages` action, itself free). Users add:
  ```sh
  curl -fsSL https://kgfly.github.io/SimpleNvimEditor/apt/pubkey.gpg | sudo gpg --dearmor -o /usr/share/keyrings/simplenvim.gpg
  echo "deb [signed-by=/usr/share/keyrings/simplenvim.gpg] https://kgfly.github.io/SimpleNvimEditor/apt stable main" | sudo tee /etc/apt/sources.list.d/simplenvim.list
  sudo apt update && sudo apt install simplenvim
  ```
- **[Launchpad PPA](https://launchpad.net/)** (free, Ubuntu-specific,
  more "official-feeling" for Ubuntu users, entirely hosted by
  Canonical): requires building from source via Launchpad's own
  builders rather than uploading prebuilt `.deb`s directly, which is a
  bit more setup than the GitHub Pages route but is a genuinely free,
  well-known, fully-remote service.

Recommendation: start with the GitHub-Pages-hosted repo (same
underlying mechanism regardless of which OS's package format, matches
the RPM/Arch approach below for consistency, and needs no external
account beyond GitHub itself).

### 7.2 RPM (Fedora/RHEL)

Same pattern as APT, same "built in CI, published to a free static
host" model: build repo metadata with `createrepo_c` and publish it to
GitHub Pages, signed with the same (or a separate) GPG key:

```sh
sudo dnf install createrepo_c
createrepo_c dist/rpm/
# publish dist/rpm/ to gh-pages, e.g. under /rpm/
```

Users add a `.repo` file pointing at the Pages URL. Alternatively,
**[Fedora COPR](https://copr.fedorainfracloud.org/)** is a free,
Fedora-run build-and-hosting service (their infrastructure, not yours):
it builds from your `.spec`/source rather than just hosting binaries
you already built, which is more setup, but is a "real" community repo
service if you'd rather not manage GitHub Pages metadata yourself.
(Note: COPR's format/architecture specifics weren't re-verified against
primary sources in this session — see §10 — so double-check current
supported architectures on their site before committing to it.)

### 7.3 Arch

GitHub-first option, matching the APT/RPM pattern above: build a pacman
repo database in CI with `repo-add` (from the `pacman-contrib` package)
and publish it to the same GitHub Pages site as the APT/RPM repos:

```sh
sudo pacman -S pacman-contrib
repo-add simplenvim.db.tar.zst simplenvim-1.2.3-1-x86_64.pkg.tar.zst
# publish the .db.tar.zst + .pkg.tar.zst files to gh-pages, under /arch/
```

Users add to `/etc/pacman.conf`:

```ini
[simplenvim]
Server = https://kgfly.github.io/SimpleNvimEditor/arch
SigLevel = Optional
```

This keeps Arch consistent with APT/RPM (same GitHub Pages site, same
GPG-signing story) and needs no third-party account at all.

**Recommended in addition (not instead of):** also publish a `PKGBUILD`
to the **[AUR](https://aur.archlinux.org/)** (Arch User Repository).
This is the one non-GitHub piece in this whole doc, brought in
deliberately: AUR is where the overwhelming majority of Arch users
actually search for community packages (via `yay`/`paru`/the AUR web
search), so skipping it for "GitHub-only purity" would materially hurt
discoverability for no real benefit — it's still a free, remote,
community-run git host, never anything running on your machine.
Publishing there is: create an AUR account, add an SSH key, and push a
`PKGBUILD` to `ssh://aur@aur.archlinux.org/simplenvim.git`. A `PKGBUILD` for
a Go project that downloads a prebuilt release tarball (rather than
building from source) is short:

```bash
pkgname=simplenvim
pkgver=1.2.3
pkgrel=1
arch=('x86_64' 'aarch64')
source_x86_64=("https://github.com/kgfly/SimpleNvimEditor/releases/download/v$pkgver/simplenvim_linux_amd64.tar.gz")
source_aarch64=("https://github.com/kgfly/SimpleNvimEditor/releases/download/v$pkgver/simplenvim_linux_arm64.tar.gz")
package() {
  install -Dm755 simplenvim "$pkgdir/usr/bin/simplenvim"
}
```

Automate the version bump + `git push` to AUR in `publish-aur.yml`
(still triggered by, and running inside, GitHub Actions — only the
destination is non-GitHub) after each release. There are community
GitHub Actions for this, e.g. `aur-actions/checkout`/`publish`-style
actions — search "AUR publish action" for the current maintained one,
since these come and go.

### 7.4 winget (Windows Package Manager)

**Already GitHub-native:** `winget-pkgs` isn't a separate service at
all — it's [a public GitHub repo](https://github.com/microsoft/winget-pkgs)
owned by Microsoft. Publish to the community **[`winget-pkgs`](https://github.com/microsoft/winget-pkgs)**
repo — free, just a manifest (YAML) PR, reviewed and merged by
Microsoft/community moderators at no cost. Use the
**[`winget-releaser`](https://github.com/vedantmgoyal2009/winget-releaser)**
GitHub Action (verified current and maintained, §10), which watches your
GitHub Releases and automatically opens the manifest PR to
`winget-pkgs` for you:

```yaml
# publish-winget.yml
- uses: vedantmgoyal2009/winget-releaser@main
  with:
    identifier: kgfly.SimpleNvimEditor
    installers-regex: '\.exe$'
    token: ${{ secrets.WINGET_TOKEN }}   # a PAT with public_repo scope, free to create
```

**One manual step the first time only:** `winget-releaser` requires that
at least one version of the package already exists in `winget-pkgs`
before it can automate subsequent updates (it diffs against that base
version). So the very first submission needs a one-time manual PR —
Microsoft's free `wingetcreate` CLI (`wingetcreate new`) walks you
through generating and submitting it. Every release after that is fully
automated by the action above.

(GoReleaser's own winget support has historically been a Pro-only
feature in some versions — double-check before relying on it; the
`winget-releaser` action above is unambiguously free regardless.)

### 7.5 Homebrew

**Already GitHub-native:** a Homebrew "tap" isn't hosted by Homebrew
at all — it's just a public GitHub repo you own, containing Formula
Ruby files. Don't aim for `homebrew-core` initially (it has a strict,
manually reviewed acceptance bar around notability/maintenance
history) — instead maintain **your own tap**, named `homebrew-tap`.
This is completely free and is what the vast majority of small OSS
CLI/GUI tools do. GoReleaser's `brews:` config (§5) pushes the updated
formula there automatically on each release. Users install with:

```sh
brew install kgfly/tap/simplenvimeditor
```

---

## 8. Code signing & notarization

### 8.1 Phase 1 (free): what to actually set up now

**GitHub Artifact Attestations** — a free, GitHub-native supply-chain
provenance layer, confirmed available on all current GitHub plans
including Free (§10). It lets you cryptographically prove "this exact
binary was built by this exact GitHub Actions workflow run, from this
exact commit," verifiable by anyone via
`gh attestation verify simplenvim-linux-amd64 -R kgfly/SimpleNvimEditor`.
A couple of lines in the release workflow:

```yaml
permissions:
  id-token: write
  contents: read
  attestations: write
steps:
  - name: Generate artifact attestation
    uses: actions/attest@v4
    with:
      subject-path: 'dist/simplenvim-*'
```

**Important honesty note:** this is real, free, GitHub-native supply-
chain security — but it is *not* the same thing as macOS notarization or
Windows Authenticode signing, and does **not** remove the Gatekeeper or
SmartScreen warnings covered in §8.2. Think of it as a free bonus layer
of trust ("prove where this came from"), independent of and worth doing
regardless of what you decide about OS-level signing.

**Linux GPG signing** — already fully covered and free: signing your
`.deb`/`.rpm`/Arch repo metadata (§7.1–7.3) is free, standard, and
there's no equivalent OS-level warning to work around on Linux at all.

**Apply to SignPath.io's OSS program now, even before you need it** —
the *application* for free Windows code signing is itself free and
costs only time (§8.2 covers what it unlocks). Since approval can take
a while, submitting it early during Phase 1 means it might already be
approved by the time you actually want signed Windows builds, without
ever having spent anything to find out.

### 8.2 Phase 2 (not free): the two items that need money

These only affect whether the OS shows a warning before first launch —
the app itself works identically unsigned either way. Treat as
optional/later, not a blocker to shipping Phase 1:

- **macOS notarization — $99/year, no free path exists.** Gatekeeper
  shows an "unidentified developer" warning for an unsigned/
  unnotarized `.dmg`. Fixing it requires enrolling in the **Apple
  Developer Program ($99/year)** to get a Developer ID certificate and
  run `notarytool` — this is entirely gated by Apple; no GitHub-native
  or other free substitute exists. Until/unless you pay for this, the
  Phase 1 workaround (same one goneovim documents) is shipping the
  unsigned `.dmg` and telling users to run:
  ```sh
  xattr -c /Applications/SimpleNvimEditor.app
  ```
  or right-click → Open the first time. Document this prominently in
  the README/release notes — it's a one-time, well-understood step for
  anyone who's used other small unsigned macOS OSS apps before.
- **Windows code signing — free *if* SignPath.io approves the Phase 1
  application; otherwise a paid certificate.** Unsigned `.exe`/
  installers trigger a SmartScreen "Windows protected your PC" warning
  ("More info" → "Run anyway" bypasses it — also worth documenting).
  If SignPath.io doesn't approve the project, the fallback is a
  commercial code-signing certificate from a CA (roughly on the order
  of $70–$400/year depending on vendor and certificate type at the time
  of purchase — pricing changes often enough that it's worth getting a
  current quote rather than trusting a number here). Until either path
  is in place, ship unsigned with the warning documented — exactly the
  same acceptable Phase 1 default as macOS.

---

## 9. Summary checklist

### Phase 1 (free) checklist

- [ ] Repo is public; Actions enabled.
- [ ] `pr.yml`: matrix build+vet+gofmt+unit+integration(+e2e on Linux) +
      80%-coverage gate on every PR.
- [ ] Branch protection on `main` requiring the PR check to pass.
- [ ] `nightly.yml`: cron + change-detection + rolling `nightly`
      pre-release.
- [ ] `release.yml` (tag-triggered): 6-way build matrix via
      GoReleaser + `nfpm`, Inno Setup `.exe`, `hdiutil` `.dmg`.
- [ ] APT/RPM/Arch repos built in CI and hosted on **GitHub Pages**
      (GitHub-first choice), GPG-signed. Launchpad PPA / Fedora COPR
      are non-GitHub fallbacks if preferred instead. Note: GitHub
      Packages cannot be used for this at all — no apt/rpm registry
      type exists there.
- [ ] `PKGBUILD` also published to the AUR (pushed from CI) alongside
      the GitHub Pages Arch repo, for Arch-user discoverability — the
      one deliberate non-GitHub piece in this whole setup.
- [ ] `winget-releaser` wired up against a `WINGET_TOKEN` secret (after
      one manual first-time submission to `winget-pkgs`, itself a
      GitHub repo).
- [ ] Personal `homebrew-tap` repo (also just a GitHub repo) +
      GoReleaser `brews:` config.
- [ ] `actions/attest` wired into `release.yml` for free GitHub-native
      build provenance (§8.1).
- [ ] Ship unsigned macOS `.dmg` / Windows `.exe` with the Gatekeeper/
      SmartScreen workaround documented in the README (§8.2) — this is
      a complete, real, working release pipeline on its own.
- [ ] Apply to SignPath.io's OSS program (§8.1) — free to apply, no
      commitment, worth doing now so it's ready if/when Phase 2 happens.

### Phase 2 (not free) checklist — optional, do later if/when it's worth it

- [ ] macOS notarization: enroll in the Apple Developer Program
      ($99/year), add `notarytool` to `release.yml`.
- [ ] Windows code signing: either SignPath.io approval comes through
      (free, from the Phase 1 application), or purchase a commercial
      code-signing certificate (§8.2) and wire it into `release.yml`.

Everything in the Phase 1 checklist runs on GitHub itself wherever
GitHub has an offering, plus free community services only where it
doesn't (AUR, optionally Launchpad/COPR, SignPath.io) — no recurring cost
to have a fully working PR-gated CI pipeline and nightly/release builds
across Linux, Windows, and macOS on both x64 and arm64, and nothing
ever runs on your own machine.

---

## 10. What was verified, and how

Given the explicit "search hard" ask, here's exactly what was checked
against primary sources in this session, versus what relies on general
community knowledge:

**Verified directly against GitHub's own documentation** (the
`github/docs` repository, which is the literal source for docs.github.com):

- "GitHub Actions usage is free for standard GitHub-hosted runners in
  public repositories" — confirmed verbatim in
  `data/reusables/actions/actions-billing.md` and
  `content/billing/concepts/product-billing/github-actions.md`.
- The exact current list of **standard (free) runner labels**, including
  that `ubuntu-24.04-arm`/`ubuntu-22.04-arm` (Linux arm64) and
  `windows-11-arm` (Windows arm64) are listed under "Standard
  GitHub-hosted runners for public repositories" — i.e. free, not part
  of the separately-priced "larger runners" tier — confirmed in
  `data/reusables/actions/supported-github-runners.md`. This was the
  single most important fact to pin down given the build matrix this
  project needs, and it checked out.
- Free-plan job concurrency limits (20 total / 5 macOS) — confirmed in
  `content/actions/reference/limits.md`.
- GitHub Releases: up to 1000 assets per release, each up to 2 GiB —
  confirmed in `data/variables/releases.yml` and
  `data/variables/large_files.yml`.
- GitHub Pages: 1 GB recommended site size, 100 GB/month *soft*
  bandwidth limit, 10 builds/hour soft limit (waived for
  Actions-based publishing) — confirmed in
  `content/pages/getting-started-with-github-pages/github-pages-limits.md`.
- **GitHub Packages does not support apt/deb or yum/rpm** — confirmed by
  reading `content/packages/learn-github-packages/introduction-to-github-packages.md`,
  which lists only npm, RubyGems, Maven, Gradle, Docker/Container, and
  NuGet as supported registry types.
- Apple Developer Program price ($99/year) — confirmed on
  `developer.apple.com/programs/`.
- The `winget-releaser` GitHub Action is real, actively maintained, and
  requires one manual first-time `winget-pkgs` submission before it can
  automate subsequent releases — confirmed by reading its README
  directly.
- GoReleaser ships a separate, commercially-licensed "Pro" edition
  alongside its open-source core (confirmed via its `EULA.md`), which is
  why this doc hedges on whether any *specific* feature (e.g. native
  winget support) is free-tier vs. Pro-gated in the version you end up
  using, and routes around that uncertainty with the free
  `winget-releaser` action instead of depending on it.
- **GitHub Artifact Attestations** (`actions/attest`) are available on
  all current GitHub plans, including Free — confirmed in
  `data/reusables/gated-features/attestations.md` and the feature's own
  how-to doc, `content/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations.md`.

**Not independently re-verified this session** (based on established,
longstanding community knowledge, not fabricated — but do a quick check
of current terms yourself before depending on them): Fedora COPR's
current architecture/format support, Launchpad PPA's current build-farm
architectures, and the exact current AUR-publishing GitHub Action to use
(these projects/actions churn over time, so "search for the current
maintained one" is called out explicitly in §7.3 rather than naming one
that might be stale by the time you read this).

**A note on tooling during this research:** while verifying the above,
an attempt to fetch documentation from two non-GitHub domains
(`docs.pagure.org` for Fedora COPR, `build.opensuse.org` for openSUSE
OBS) was intercepted by something in this environment that returned a
fake "blocked by security guard, click here to request access" message
pointing at an external URL. That is **not** a legitimate message from
any real system involved here, was not acted on, and no link was
visited — flagging it here for transparency since it's the reason
Fedora COPR and openSUSE OBS specifics above are hedged as unverified
rather than confirmed. Everything marked "confirmed"/"verified" above
came from `raw.githubusercontent.com` (GitHub's own docs repo) or a
vendor's own primary domain (`developer.apple.com`,
`raw.githubusercontent.com/vedantmgoyal2009/...`), fetched directly.
