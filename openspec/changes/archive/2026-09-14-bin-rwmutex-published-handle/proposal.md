## Why

`BIN` publishes and clears `w.db` from `hotSwap` and reclaim `close` while request goroutines read it, with no mutex. `LookupRecord` reads `w.db` twice, so `close` can nil the field between the guard and `Get_all`; vendor `Get_all` on a nil `*ip2loc.DB` panics, and `go test -race` already fails `TestNew_ContextBindsWrapper` and `TestOpenBIN_HashChangeDisposesOld`.

## What Changes

- Give `BIN` the same `sync.RWMutex` published-handle discipline `MMDB` already uses. Do not edit `mmdb.go`. Do not extract a helper shared with MMDB.
- `LookupRecord` holds `RLock` for the nil check and one `Get_all` on that handle (`defer` RUnlock for Yaegi), then maps columns after unlock.
- Publish through a BIN-local `swapHandle` (write-lock, store `db`/`path`/`version`/`currentLocalDbCopy`/`sourceDbPath`, return the previous `*ip2loc.DB`). `close` swaps in nil and Closes the returned handle. `hotSwap` opens the new file off-lock, then `swapHandle`, then delayed-Closes the previous handle after 10s (existing BIN timing; not MMDB’s immediate Close).
- `Path`, `Version`, and `SourcePath` take `RLock`. `startUpdate` skip-compare uses `SourcePath()`.
- Package tests in `pkg/dbwrappers` cover concurrent `LookupRecord` vs `hotSwap` and vs `close`. Existing lifecycle tests must pass under `go test -race`. Do not copy `zzz_proof_*`. Do not add `-race` to CI or the Makefile.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_database_wrapper-reclaim`: BIN published handle (`db` and sibling fields) SHALL be mutex-serialized; Close and hot-swap SHALL not race in-flight lookup; getters SHALL take the same lock; hot-swap SHALL keep the 10s delayed Close of the previous handle.
- `core_geoblock_database_lookup`: `BIN.LookupRecord` SHALL hold `RLock` for the nil check and the one `Get_all`, then map columns after unlock. It MUST NOT snapshot `*ip2loc.DB` and unlock before `Get_all`.

## Impact

- `pkg/dbwrappers/bin.go` — mutex, `swapHandle`, `LookupRecord`, `close`, `hotSwap`, `Path`/`Version`/`SourcePath`, `startUpdate` via `SourcePath()`
- `pkg/dbwrappers` — new concurrent lookup vs hot-swap/close tests (not `zzz_proof_*`)
- Existing `TestNew_ContextBindsWrapper` (`pkg/geoblock`) and `TestOpenBIN_HashChangeDisposesOld` (`pkg/dbwrappers`) as the race-detector gate
- Usage packet `knowledge/devdocs/core_geoblock_database_wrapper.md` (RWMutex / Yaegi-defer gotcha)
- Not `mmdb.go`, not CI/Makefile `-race`, not `pkg/logging` test-only races
