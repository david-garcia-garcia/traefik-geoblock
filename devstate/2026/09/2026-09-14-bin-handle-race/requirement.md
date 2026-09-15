# Requirement
IssueKey: 2026-09-14-bin-handle-race

## Problem
IP2Location BIN wrappers publish and clear `w.db` from `hotSwap` and `close` while request goroutines read it. There is no mutex. `LookupRecord` reads `w.db` twice, so `close` can nil the field between the guard and `Get_all`. The race detector already fails two existing tests. Production risk: undefined country data on allow/block, and a nil-handle panic in Traefik's handler chain.

## Current (code)
- `pkg/dbwrappers/bin.go` `BIN` has `db`, `path`, `version`, `currentLocalDbCopy`, `sourceDbPath` and no `sync.RWMutex`.
- `pkg/dbwrappers/bin.go` `hotSwap` writes `w.db` / `w.path` / `w.version` / `w.currentLocalDbCopy` / `w.sourceDbPath` with no lock, then closes the old handle after 10s.
- `pkg/dbwrappers/bin.go` `LookupRecord` reads `w.db` for a nil check, then reads `w.db` again for `Get_all`.
- `pkg/dbwrappers/bin.go` `close` stops the updater, `Close`s `w.db`, then sets `w.db = nil` with no lock. `Path` / `Version` / `SourcePath` also read those fields unlocked.
- `pkg/dbwrappers/mmdb.go` `MMDB` has `sync.RWMutex`. `swapReader` publishes `db`/`path` under the write lock. `Lookup` holds `RLock` for the nil check and `db.Lookup`. `Path` and `close` use the same lock.
- `vendor/github.com/ip2location/ip2location-go/v9/ip2location.go` `Get_all` calls `query`, which reads `d.metaok` with no nil-receiver guard — `Get_all` on a nil `*ip2loc.DB` panics.
- `pkg/geoblock/plugin_lifecycle_test.go` `TestNew_ContextBindsWrapper` cancels the plugin context and looks up while reclaim `Hooks.Close` runs `BIN.close`.
- `pkg/dbwrappers/reclaim_test.go` `TestOpenBIN_HashChangeDisposesOld` disposes one BIN generation while lookups still run. Bug-hunt report: both fail under `go test -race`.
- `.github/workflows/ci.yml` test job is `go test -v ./...`. `Makefile` test target is `go test -v -cover .`. Neither passes `-race`.
- `pkg/logging/logging_test.go` `TestStdoutWriter_Write` / `TestNew_JSONFormat` race a capture buffer under `-race` (test-only; product logging not implicated).

## Desired
- Give `BIN` the same `sync.RWMutex` discipline `MMDB` already uses for the published handle.
- `LookupRecord` takes the handle once (read lock + one use of that handle for `Get_all`), matching `MMDB.Lookup`.
- Existing tests that fail under `-race` today (`TestNew_ContextBindsWrapper`, `TestOpenBIN_HashChangeDisposesOld`) must pass.
- Write proper product tests for the concurrent lookup vs hot-swap / close paths. Do not copy caller-workspace `zzz_proof_*` files.
- Do not change `MMDB` unless a shared helper requires a symmetric edit.
- Adding `-race` to CI only if it is a one-line flag that already works. Otherwise note it; do not turn this ticket into a CI overhaul.

## Affected
- `pkg/dbwrappers/bin.go` (mutex, publish, lookup, close; Path/Version/SourcePath if they share the published fields)
- `pkg/dbwrappers/mmdb.go` only if extracting the same helper
- Existing lifecycle tests above; new BIN concurrency tests next to the wrapper
- Re-measure with `go test -race` (docker `golang:1.25` `-e GOFLAGS=-mod=vendor` when the Windows host has no gcc)

## Out of scope
- F-2 through F-9 (CIDR family trees, empty `bypassHeaders`, updater `Stop` join / F-4, `/0` allow, corrupt dated file, `countryHeader` hijack, empty hop list, block-mode `Lookup` panic)
- Copying or committing `zzz_proof_*` proof files
- CI / Makefile `-race` overhaul when it is not a one-line flag that already works
- Logging test-only races in `pkg/logging/logging_test.go`

## Unknowns
- This prepare pass did not re-run docker `-race`; implement must re-measure. Windows host may lack gcc; use `docker golang:1.25` with `GOFLAGS=-mod=vendor` when local `-race` fails.
- `go test -v -race ./...` on GitHub `ubuntu-latest` is one workflow line, but it is not already green: logging tests fail under `-race`, and Yaegi tests skip under `-race`. Parked as a follow-up note.

## Tensions
- Ticket names a new `swapHandle` helper. Dest already has `MMDB.swapReader` for the same publish job. Matching MMDB may mean reusing that shape/name pattern, not a second helper name.
- `MMDB.Lookup` holds `RLock` for the vendor lookup rather than copying the reader and unlocking first. "Read-once local" must not invent a weaker BIN pattern than MMDB unless explore records a reshape.
- Suggested CI `-race` vs "do not expand into a CI overhaul": dest is not one-line-green, so this run notes it and does not take it.
