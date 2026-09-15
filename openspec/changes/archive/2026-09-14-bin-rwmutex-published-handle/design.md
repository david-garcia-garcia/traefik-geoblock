## Context

See proposal.md. Dest `BIN` has no `sync.RWMutex`; `hotSwap` and `close` write `w.db` unlocked; `LookupRecord` nil-checks `w.db` then calls `w.db.Get_all`. `MMDB` already has `mu`, `swapReader`, and `Lookup` that holds `RLock` across the vendor call (`defer` Unlock because Yaegi recovers panics). Vendor `Get_all` has no nil-receiver guard. `GetRemoteIPs` remains the client-address owner; `LookupRecord` only consumes the `ip` string the plugin already chose.

## Goals / Non-Goals

**Goals:**
- Match MMDB’s published-handle discipline on BIN only: mutex, publish helper, read lock across the vendor lookup, locked getters.
- Keep BIN’s 10s delayed Close of the previous hot-swap handle.
- Package tests for concurrent lookup vs hot-swap and vs close; existing lifecycle tests green under `-race`.

**Non-Goals:**
- Edit `mmdb.go` or extract a helper shared with MMDB.
- Lock `updater` (MMDB leaves `sleep`/`wake`/`startUpdate` unlocked).
- Delete `currentLocalDbCopy`.
- Copy `zzz_proof_*`. Add `-race` to CI or the Makefile.
- F-2 through F-9; logging test-only races.

## Decisions

1. **BIN-local `swapHandle`** — Same shape as `MMDB.swapReader`: write-lock, store `db`/`path`/`version`/`currentLocalDbCopy`/`sourceDbPath`, return previous `*ip2loc.DB`, `defer` Unlock. Not named `swapReader` (that is `*maxminddb.Reader`). Alternative: shared helper — rejected; Desired forbids changing MMDB unless a shared helper requires it.

2. **Hold `RLock` across `Get_all`** — Copying `*ip2loc.DB` and unlocking first lets `close` `Close` the vendor handle under a live `query`. MMDB already holds the read lock for `db.Lookup`. Column mapping runs after unlock. Alternative: snapshot-and-unlock — rejected as weaker than MMDB.

3. **Keep 10s delayed Close after hot-swap** — Mutex already waits in-flight `Get_all` before the swap publishes. Immediate Close (MMDB) is a reshape of existing BIN timing no criterion named. Recorded on `devstate/deviations.md`. `close` still Closes the unpublished handle immediately (swap nil, then Close).

4. **`initialize` stays unlocked** — Single-threaded before the reclaim table publishes the pointer. Alternative: route init through `swapHandle` — unnecessary; no concurrent reader exists yet.

5. **Locked getters; `startUpdate` uses `SourcePath()`** — Those fields share the published cluster. MMDB `Path` already `RLock`s. `startUpdate` today reads `w.sourceDbPath` unlocked.

6. **Tests next to the wrapper** — New package tests in `pkg/dbwrappers`. Keep `TestNew_ContextBindsWrapper` and `TestOpenBIN_HashChangeDisposesOld` as the race-detector gate. Do not commit `zzz_proof_*`.

## Risks / Trade-offs

- [RLock held for vendor `Get_all` duration] → Mitigation: same as MMDB `Lookup`; BIN `Get_all` is already the request-path cost.
- [10s delayed Close keeps the previous file mapped after swap] → Mitigation: existing BIN behavior; mutex only serializes publish. Do not shorten it in this change.
- [Windows host has no gcc for `-race`] → Mitigation: `docker run --rm -e GOFLAGS=-mod=vendor -w /src golang:1.25 go test -race` on `pkg/dbwrappers` and `pkg/geoblock`.
- [Yaegi recovers panics without running a trailing Unlock] → Mitigation: `defer` Unlock/RUnlock on every lock path, matching MMDB comments.

## Migration Plan

Ship in the next plugin build. No config or operator migration. Race-detector failures on the two named tests go away; lookup results stay the same.

## Open Questions

Ticket questions live on `devstate/explore.md`.
