# Requirement
IssueKey: 2026-09-16-zzz-test-prefix

## Problem
First-party `*_test.go` files sit next to production `.go` files with the same stem, so they do not cluster in the explorer. The ticket asks to prefix those test filenames with `zzz_` and keep the `_test.go` suffix so `go test` still discovers them.

## Current (code)
- Thirty first-party `*_test.go` files on `origin/master` (`509e6c0`) use the unprefixed stem, e.g. `pkg/dbwrappers/bin_test.go`, `plugin_instance_test.go`, `pkg/geoblock/plugin_policy_test.go`. None already start with `zzz_`.
- `go test` discovery is the `_test.go` suffix (`knowledge/devdocs/core_geoblock_test-harness.md`). CI runs `go test -v ./...` (`.github/workflows/ci.yml`); no workflow glob pins a test basename.
- Usage packet `knowledge/devdocs/core_geoblock_test-harness.md` names the current basenames (`plugin_instance_test.go`, `pkg/geoblock/plugin_*_test.go`, `pkg/*/*_test.go`).
- Vendor has no `*_test.go` (`vendor/` omitted by `go mod vendor`). Pester cases live in `scripts/integration-tests.Tests.ps1`, not Go test files.

## Desired
Rename every first-party Go package test file so the basename is `zzz_<oldstem>_test.go` (example: `bin_test.go` → `zzz_bin_test.go`). Keep package names, test function names, and product runtime behavior unchanged. Open a pull request.

## Affected
- `plugin_instance_test.go`
- `pkg/logging/logging_test.go`
- `pkg/geoblock/plugin_policy_test.go`
- `pkg/geoblock/plugin_test.go`
- `pkg/geoblock/plugin_mode_test.go`
- `pkg/geoblock/plugin_observe_test.go`
- `pkg/geoblock/plugin_ipheaders_test.go`
- `pkg/geoblock/plugin_lifecycle_test.go`
- `pkg/geoblock/plugin_filters_test.go`
- `pkg/geoblock/plugin_config_test.go`
- `pkg/geoblock/perf_test.go`
- `pkg/geoblock/ip_blocks_test.go`
- `pkg/fileutils/fileutils_test.go`
- `pkg/dbwrappers/reclaim_test.go`
- `pkg/dbwrappers/mmdb_extract_test.go`
- `pkg/dbwrappers/mmdb_test.go`
- `pkg/dbwrappers/fields_test.go`
- `pkg/dbwrappers/geoip2_test.go`
- `pkg/dbwrappers/ipinfo_test.go`
- `pkg/dbwrappers/delayed_close_test.go`
- `pkg/dbwrappers/bin_handle_test.go`
- `pkg/dbwrappers/bin_test.go`
- `pkg/dbwrappers/bin_record_test.go`
- `pkg/dbutils/httpget_test.go`
- `pkg/dbutils/dbutils_test.go`
- `pkg/dbsource/updater_test.go`
- `pkg/dbsource/update_test.go`
- `pkg/dbsource/resolve_test.go`
- `pkg/dbprovider/provider_test.go`
- `pkg/dbprovider/combine_test.go`

## Out of scope
- Product runtime behavior, package names, or test function names.
- Vendor `*_test.go` (none on dest) and upstream utilities tests.
- Pester / compose integration files (`scripts/integration-tests.Tests.ps1`).
- Rewriting `knowledge/devdocs/core_geoblock_test-harness.md` or archived OpenSpec paths that cite old basenames (inferred; ticket is filename convention only).
- Adding or copying `zzz_proof_*` hunt files from the caller workspace.

## Unknowns
- Whether any later usage or spec leaf must be updated for the new basenames (ticket did not ask).
- Whether a new first-party `*_test.go` appears on dest after this dump.

## Tensions
- Earlier tickets forbade committing `zzz_proof_*` hunt names and required ordinary `_test.go` product tests. This ask is `zzz_` + original stem + `_test.go` (e.g. `zzz_bin_test.go`), not `zzz_proof_*`.
