# Build SimpleNvimEditor and run it from build\ without installing it.
# The title-bar and taskbar icons are set from the embedded PNGs at runtime,
# so they need no installer.
$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$exe = Join-Path $root 'build\simplenvim.exe'

Write-Host '==> Building'
New-Item -ItemType Directory -Force (Join-Path $root 'build') | Out-Null
Push-Location (Join-Path $root 'src')
try {
    go build -o $exe ./cmd/simplenvim
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
    Pop-Location
}

& $exe @args
