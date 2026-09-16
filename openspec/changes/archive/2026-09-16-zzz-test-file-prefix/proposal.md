## Why

First-party `*_test.go` files sit next to the production `.go` they cover, so they do not cluster in the explorer. The ask is a basename prefix so tests sort together. `go test` already keys on the `_test.go` suffix.

## What Changes

- Rename every first-party Go package test file so the basename is `zzz_<oldstem>_test.go` (example: `bin_test.go` → `zzz_bin_test.go`).
- Keep package names, `Test*` / `Benchmark*` names, and product runtime behavior unchanged.
- Do not rename Pester files, vendor trees, or hunt `zzz_proof_*` names.
- Do not rewrite usage packets or archived OpenSpec paths that cite old stems.

## Capabilities

### New Capabilities

### Modified Capabilities

No spec-level behavior change. Plugin, catalog, and lookup requirements stay as specified. This change is a filename convention on existing package tests. `skip_specs: true` is set on this change.

## Impact

- Thirty first-party `*_test.go` files listed in `devstate/requirement.md` Affected (re-list at apply; exclude `vendor/`)
- CI still `go test -v ./...` (`.github/workflows/ci.yml`); no workflow glob pins a test basename
- Usage packet `knowledge/devdocs/core_geoblock_test-harness.md` Key-files stems stay stale this change (Out of scope)
