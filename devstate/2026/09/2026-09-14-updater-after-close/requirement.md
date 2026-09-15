# Requirement
IssueKey: 2026-09-14-updater-after-close

## Problem
`Updater.Stop` does not join a tick already inside HTTP download. That tick still calls `onUpdate`, so BIN `hotSwap` and MMDB `open` publish a new handle on a generation the reclaim table has already disposed. Lookups succeed after Close; BIN also leaves a new temp copy in `os.TempDir()`. The first tick runs as soon as `Start` launches, so the window is every URL-backed wrapper start, not only the 24h tick.

## Current (code)
- `pkg/dbsource/updater.go` `Start`: creates ticker and `stop`, then a goroutine that calls `tick` immediately and loops on `ticker.C` vs `stop`. `Start` does not wait for that first tick.
- `pkg/dbsource/updater.go` `tick`: `UpdateIfNeeded` then `onUpdate(path)` with no read of `stop` after the download returns.
- `pkg/dbsource/updater.go` `Stop`: `ticker.Stop` and close `stop` if still open. No `done` channel, no wait for the goroutine. A nil receiver returns.
- `pkg/dbsource/update.go` `Update` / `UpdateIfNeeded`: blocking `dbutils.HTTPGet` (`pkg/dbutils/httpget.go`, 30-minute client timeout, no `context.Context`). `Stop` cannot cancel that GET today.
- `pkg/dbwrappers/bin.go` `startUpdate`: `dbsource.Start` with a callback that skips empty/`sourceDbPath` then calls `hotSwap`. No closed check.
- `pkg/dbwrappers/bin.go` `hotSwap`: always opens a temp copy and assigns `w.db` / `w.path` / `w.currentLocalDbCopy`. `close` (`bin.go`) calls `updater.Stop` then `w.db.Close()` and sets `w.db = nil`. It does not set a closed flag and does not refuse a later `hotSwap`.
- `pkg/dbwrappers/bin.go` `AllowMissing`: `initialize` can return with `w.db == nil` while still waiting for the first download, so `db == nil` is not “closed”.
- `pkg/dbwrappers/mmdb.go` `startUpdate`: callback skips empty/`Path()` then `open`. `close` calls `updater.Stop` then `swapReader(nil, "")`. `open` has no closed check, so a post-close `onUpdate` publishes a new reader.
- `pkg/dbwrappers/bin.go` `sleep` / `wake` and `pkg/dbwrappers/mmdb.go` `sleep` / `wake`: Sleep is `updater.Stop`; Wake is `startUpdate`. Without a join, Wake can start a second goroutine while the first tick is still in GET.
- `pkg/dbwrappers/bin_test.go` `TestOpenBIN_HotSwap` and `pkg/dbwrappers/mmdb_test.go` `TestOpenMMDB_OpenIsHotSwap`: sequential swap only. `pkg/dbsource` has no `Start`/`Stop` test. No product test holds the download until after Close.
- `openspec/specs/core_geoblock_database_wrapper-reclaim/spec.md`: Sleep SHALL stop the updater; Close SHALL stop the updater and close the file or reader. It does not say Stop joins or that a disposed generation must stay disposed.

## Desired
A disposed generation stays disposed. `Updater.Stop` is synchronous: join the ticker goroutine. `tick` does not call `onUpdate` after stop. BIN and MMDB ignore an update that arrives after close, so a missed join cannot reopen. One owner for the join (`Updater`). Smallest durable delta. Product tests for delayed-download-until-after-close on BIN and MMDB (not `zzz_proof_*` names). Do not take F-1’s BIN `LookupRecord` mutex unless explore proves the ignore-after-close check cannot be made safe without it; if `hotSwap` must gain an ignore-after-close check, do not expand into the unsynchronized lookup race (`2026-09-14-bin-handle-race`).

## Affected
- `pkg/dbsource/updater.go` (`Start`, `tick`, `Stop`)
- `pkg/dbwrappers/bin.go` (`startUpdate` / `hotSwap` / `close`)
- `pkg/dbwrappers/mmdb.go` (`startUpdate` / `open` / `close`)
- New product tests next to those packages (BIN and MMDB delayed-download-until-after-close)
- Later: `openspec/specs/core_geoblock_database_wrapper-reclaim/spec.md` (Close/Sleep join wording)

## Out of scope
- F-1 / F-1b BIN `LookupRecord` vs `hotSwap`/`close` data race (`2026-09-14-bin-handle-race`)
- F-2 CIDR family mix, F-3 empty bypass, F-5 `/0` allow, F-6 corrupt dated file, F-7 countryHeader hijack, F-8 empty hop list, F-9 nil `Lookup` panic
- Adding `context` cancellation to `HTTPGet` unless explore shows join-without-cancel cannot meet Desired
- Copying hunt `zzz_proof_*` filenames or committing those hunt files

## Unknowns
- Whether BIN ignore-after-close can be a closed flag (or equivalent) without F-1’s `sync.RWMutex`; `db == nil` cannot be that flag because `AllowMissing` starts with a nil handle
- Whether `Stop` join should wait out an in-flight GET (up to `HTTPGetTimeout`) or this change must also cancel `HTTPGet` so reclaim Close/Sleep cannot block for minutes
- Whether MMDB can treat post-close `db == nil` plus empty path as closed, or needs its own closed flag so a first-open path cannot be confused with dispose

## Tensions
- Ticket: do not take F-1’s BIN mutex unless explore proves ignore-after-close is unsafe without it. Writing a closed flag in `close` while `hotSwap` reads it without a lock is another race; an `atomic.Bool` might be enough and would still leave F-1’s `LookupRecord` race on `2026-09-14-bin-handle-race`.
- `wrapper-reclaim` Sleep is `Stop`. A synchronous join makes Sleep wait for an in-flight GET; today’s Sleep returns immediately and can overlap Wake.
- Hunt proof tests live in the caller tree as `zzz_proof_*`; this ticket forbids those names and asks for product tests of the same delayed-download-until-after-close scenario.
