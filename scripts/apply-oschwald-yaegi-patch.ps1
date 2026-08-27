#!/usr/bin/env pwsh
# Re-apply after `go mod vendor`. See patches/oschwald-maxminddb-golang/README.md
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$vendor = Join-Path $root "vendor/github.com/oschwald/maxminddb-golang"
$overlay = Join-Path $root "patches/oschwald-maxminddb-golang/reader_mmap.go.overlay"
if (-not (Test-Path $vendor)) {
    throw "vendor oschwald package missing; run go mod vendor first"
}
Copy-Item -Force $overlay (Join-Path $vendor "reader_mmap.go")
Remove-Item -Force -ErrorAction SilentlyContinue (Join-Path $vendor "mmap_unix.go")
Remove-Item -Force -ErrorAction SilentlyContinue (Join-Path $vendor "mmap_windows.go")
Write-Host "Applied oschwald Yaegi overlay (Open is ReadFile; mmap files removed)."
