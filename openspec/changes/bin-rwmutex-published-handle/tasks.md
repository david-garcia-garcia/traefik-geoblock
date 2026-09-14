## 1. BIN published-handle mutex

- [x] 1.1 Add `sync.RWMutex` on `BIN` and a BIN-local `swapHandle` (write-lock, store `db`/`path`/`version`/`currentLocalDbCopy`/`sourceDbPath`, return previous `*ip2loc.DB`, `defer` Unlock). Do not edit `mmdb.go`.
- [x] 1.2 Route `hotSwap` publish through `swapHandle`; keep the 10s delayed Close of the previous handle. Leave `initialize` unlocked.
- [x] 1.3 Route `close` through `swapHandle(nil, …)` then Close the returned handle. Leave `updater` unlocked.

## 2. Lookup and getters

- [x] 2.1 Hold `RLock` in `LookupRecord` for the nil check and one `Get_all` (`defer` RUnlock); map columns after unlock. Do not snapshot-and-unlock.
- [x] 2.2 Take `RLock` in `Path`, `Version`, and `SourcePath`. Compare in `startUpdate` via `SourcePath()`.

## 3. Tests

- [x] 3.1 Add package tests in `pkg/dbwrappers` for concurrent `LookupRecord` vs `hotSwap` and vs `close`. Do not copy `zzz_proof_*`.
- [ ] 3.2 Keep `TestNew_ContextBindsWrapper` and `TestOpenBIN_HashChangeDisposesOld` as the race-detector gate (no rewrite unless they fail for an unrelated reason).

## 4. Usage

- [x] 4.1 Update `knowledge/devdocs/core_geoblock_database_wrapper.md` with the RWMutex / Yaegi-`defer` Unlock gotcha and the 10s delayed hot-swap Close.

## 5. Measure

- [ ] 5.1 Re-run `go test -race` on `pkg/dbwrappers` and `pkg/geoblock` (docker `golang:1.25` `-e GOFLAGS=-mod=vendor` when the host has no gcc) and confirm the two named tests pass.
