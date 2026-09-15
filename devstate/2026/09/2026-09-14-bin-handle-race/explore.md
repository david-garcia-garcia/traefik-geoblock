# Explore
IssueKey: 2026-09-14-bin-handle-race
Verdict: in progress

## Concepts

**BIN published handle.** `pkg/dbwrappers/bin.go` `BIN` holds `db *ip2loc.DB`, `path`, `version`, `currentLocalDbCopy`, `sourceDbPath`. `hotSwap` and `close` write `w.db` with no mutex. `LookupRecord` reads `w.db` for a nil check, then reads it again for `Get_all`. Reproduced: `docker run --rm -e GOFLAGS=-mod=vendor -w /src golang:1.25 go test -race -count=1 -run TestOpenBIN_HashChangeDisposesOld|TestNew_ContextBindsWrapper ./pkg/dbwrappers ./pkg/geoblock` — both FAIL. Race: `LookupRecord` `bin.go:277` vs `close` `bin.go:345` (`w.db = nil`) from reclaim `Hooks.Close`.

```
request goroutine                         reclaim expire / hotSwap
----------------                         -------------------------
LookupRecord: if w.db == nil             close: w.db.Close(); w.db = nil
LookupRecord: w.db.Get_all(ip)           hotSwap: w.db = newDB  (unlocked)
              ↑ torn or nil receiver
```

**MMDB published handle.** `pkg/dbwrappers/mmdb.go` already owns the target discipline: `sync.RWMutex`; `swapReader` write-locks, publishes `db`/`path`, returns the previous `*maxminddb.Reader`; `Lookup` RLock for nil-check + `db.Lookup`; `Path` and `close` use the same lock. Yaegi comment: `defer` Unlock because a recovered panic would skip a trailing Unlock. Desired: BIN matches this, not a weaker snapshot.

**Vendor `Get_all`.** `vendor/github.com/ip2location/ip2location-go/v9/ip2location.go` `Get_all` → `query`. `query` reads `d.metaok` with no nil-receiver guard (`ip2location.go:920`). `Get_all` on a nil `*ip2loc.DB` panics. Research index has download codes only (`knowledge/research/index_ext_ip2location.md`); this fact is the vendored source, not a new download-API finding.

**Wrapper vs plugin.** Production BIN lookup is one Bind in `pkg/geoblock/plugin.go` (`openCatalogRow` → `bin.LookupRecord`). Client address stays on `Plugin.GetRemoteIPs` (`pkg/geoblock/ip.go`); `LookupRecord` receives an already-chosen `ip string`. Reclaim `Hooks.Close` is the dispose path the lifecycle tests race.

**Call sites of BIN publish/read (searched `*.go` excluding `vendor/` for `OpenBIN`, `LookupRecord`, `.hotSwap`, `w.close`, `Path()`, `Version()`, `SourcePath()`).** Production: `plugin.go` Bind (1 LookupRecord), `bin.go` `startUpdate` → `hotSwap` (1), `OpenBIN` `Hooks.Close` (1), `startUpdate` reads `w.sourceDbPath` unlocked (1). Tests: `pkg/dbwrappers/bin_test.go`, `bin_record_test.go`, `reclaim_test.go`; plugin lifecycle via `Plugin.Lookup`. MMDB `swapReader` callers: `open` and `close` in `mmdb.go` only (2). All enumerated and migratable in this change.

## Decisions

- Give `BIN` `sync.RWMutex` and the same publish/read discipline as `MMDB`. Do not edit `mmdb.go`. Do not extract a shared helper (Desired: “Do not change MMDB unless a shared helper requires a symmetric edit”).
- `LookupRecord` holds `RLock` for the nil-check and one `Get_all` on `w.db`, then maps columns after unlock — matching `MMDB.Lookup` (vendor call under the read lock; field copy is after). Use `defer` RUnlock (Yaegi). Do not copy `*ip2loc.DB` and unlock before `Get_all`: `close` would `Close` the vendor handle under a live `query`.
- Publish through a BIN-local `swapHandle` with the `swapReader` shape: write-lock, store `db`/`path`/`version`/`currentLocalDbCopy`/`sourceDbPath`, return the previous `*ip2loc.DB`, `defer` Unlock. `close` swaps in nil and Closes the returned handle. `hotSwap` opens the new file off-lock, then `swapHandle`, then the existing 10s delayed `Close` of the old handle (do not switch to MMDB’s immediate Close). `initialize` stays unlocked (single-threaded before the table publishes the pointer).
- `Path`, `Version`, and `SourcePath` take `RLock` (they share the published fields). `startUpdate` skip-compare uses `SourcePath()`, not a raw `w.sourceDbPath` read.
- Leave `updater` unlocked, matching MMDB `sleep`/`wake`/`startUpdate`/`close`. Not this ticket.
- Keep `currentLocalDbCopy` (written only; not read). Publish it under `swapHandle` if still assigned there. Do not delete it.
- Product tests: new package tests in `pkg/dbwrappers` for concurrent `LookupRecord` vs `hotSwap` and vs `close`. Do not copy caller-workspace `zzz_proof_*`. Existing `TestNew_ContextBindsWrapper` and `TestOpenBIN_HashChangeDisposesOld` must pass under `go test -race` (docker `golang:1.25` `GOFLAGS=-mod=vendor` when the host has no gcc).
- Do not add `-race` to CI or the Makefile. Follow-up already noted: `knowledge/debt/2026-09-14-ci-go-race-detector.md` / `issues.md`.
- Propose folds the mutex contract onto existing specs (`core_geoblock_database_wrapper-reclaim` Close vs in-flight lookup, `core_geoblock_database_lookup` already requires one `Get_all`). Update wrapper usage (`knowledge/devdocs/core_geoblock_database_wrapper.md`) with the RWMutex / Yaegi-defer gotcha in this change. No new spec family.
- This work does not set or reconstruct client address, user, tenant, Host, or trust hop. `GetRemoteIPs` remains the owner; `LookupRecord` only consumes the `ip` the plugin already chose.

## Open questions

- Q: Does `LookupRecord` hold `RLock` for `Get_all`, or snapshot `*ip2loc.DB` and unlock first?
  Rank: bounded asked — existing `BIN.LookupRecord` (1 production Bind in `pkg/geoblock/plugin.go`; tests in `bin_test.go`, `bin_record_test.go`, `reclaim_test.go`, `plugin_lifecycle_test.go`); Desired “takes the handle once (read lock + one use of that handle for Get_all), matching MMDB.Lookup”
  Decision: resolved — no mutex on the request path. Copy `w.db` once, then `Get_all` on that local. close Closes the file and does not nil the pointer (nil `Get_all` panics). Stale Path/Version/SourcePath during swap is accepted.
  By: implement

- Q: What publishes the BIN handle — `swapHandle`, `swapReader`, or a helper shared with MMDB?
  Rank: additive asked — new method this change creates; Tension “Matching MMDB may mean reusing that shape/name pattern, not a second helper name”; Desired “Do not change MMDB unless a shared helper requires a symmetric edit”
  Decision: assumed — BIN-local `swapHandle` with `MMDB.swapReader` shape. Do not name it `swapReader` (that is `*maxminddb.Reader`). Do not extract a shared helper; leave `mmdb.go` unchanged.
  By: propose

- Q: Do `Path` / `Version` / `SourcePath` take the same mutex as `db`?
  Rank: bounded asked — Affected “Path/Version/SourcePath if they share the published fields”; call sites `bin_test.go` Version/Path/SourcePath and `bin.go` `startUpdate` `w.sourceDbPath`; MMDB `Path` already RLock
  Decision: resolved — no. Getters are unlocked. A stale Path/Version/SourcePath during hot-swap is accepted. `startUpdate` still compares via `SourcePath()`.
  By: implement

- Q: After `swapHandle`, does BIN still delay-Close the old handle by 10s, or Close immediately like MMDB?
  Rank: bounded incidental — `hotSwap` post-swap Close goroutine only (production `startUpdate` + tests `TestOpenBIN_HotSwap` / `TestOpenBIN_InitLogsDatedCopy`); no criterion names the delay; 10s is existing BIN behavior
  Decision: assumed — keep the 10s delayed Close of the old handle. Mutex already waits in-flight `Get_all` before swap. Do not reshape hot-swap timing to match MMDB.
  By: propose

- Q: Where do the new concurrent lookup vs hot-swap/close tests live?
  Rank: additive asked — Desired “Write proper product tests for the concurrent lookup vs hot-swap / close paths” and “new BIN concurrency tests next to the wrapper”; “Do not copy caller-workspace zzz_proof_*”
  Decision: assumed — add package tests in `pkg/dbwrappers` (not `zzz_proof_*`, not `pkg/geoblock`). Keep the two existing lifecycle tests as the race-detector gate.
  By: propose

- Q: Add `go test -race` to CI / Makefile in this change?
  Rank: additive incidental — Out of scope “CI / Makefile -race overhaul when it is not a one-line flag that already works”; Unknowns park logging-test races
  Decision: assumed — do not add the flag. Follow-up remains `knowledge/debt/2026-09-14-ci-go-race-detector.md`.
  By: propose
