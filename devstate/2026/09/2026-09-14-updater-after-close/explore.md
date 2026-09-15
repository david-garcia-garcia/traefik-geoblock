# Explore

## Concepts

- **Updater** (`pkg/dbsource/updater.go`): keep-current loop for one source. `Start` launches a goroutine that always runs `tick` once, then selects on the 24h ticker vs `stop`. `tick` calls `UpdateIfNeeded` then `onUpdate` with no read of `stop` after the GET returns. `Stop` stops the ticker and closes `stop`; it does not join and cannot cancel `dbutils.HTTPGet` (`HTTPGetTimeout` = 30m, no `context`).
- **Wrapper generation**: reclaim `Hooks.Close` is `BIN.close` / `MMDB.close`. Sleep is `updater.Stop`. A URL-backed `newBIN` / `newMMDB` starts the updater before return, so the first GET can already be in flight when Close/Sleep runs.
- **Resurrection**: a tick that finishes after Close still publishes. BIN `hotSwap` always assigns `w.db` and leaves a new `os.TempDir()` copy (`bin_<key>_<unixNano>.BIN`). MMDB `open` has no closed check; `close` already cleared path via `swapReader(nil, "")`.
- **`db == nil` is not closed (BIN)**: `AllowMissing` can return from `initialize` with `w.db == nil` while the first download is still pending. MMDB has no AllowMissing; `db == nil` plus empty path is also the zero value before the first `open`, so it cannot mean disposed.
- **Join vs cancel**: reclaim Sleep/Close run outside `t.mu` but park that key (`slotBusy` for Sleep; Close unmaps first unless `EnforceCloseBeforeOpen`). A synchronous join waits out an in-flight GET on that key only. Other keys stay usable. Join-without-cancel meets Desired; `HTTPGet` context stays out of scope.
- **F-1 leftover**: BIN `LookupRecord` vs `hotSwap`/`close` still unsynchronized. Ignore-after-close does not need that mutex (`atomic.Bool` plus re-check before publish). Owned by `2026-09-14-bin-handle-race`.

## Decisions

- Failure reproduced on this worktree’s master code. Delayed-download-until-after-close: BIN and MMDB both published a live handle after `close` and served `8.8.8.8`. Throwaway tests deleted after the run; do not commit `zzz_proof_*`.
- One owner for the join: `Updater.Stop` waits for the ticker goroutine. `tick` must not call `onUpdate` after stop (check `stop` after `UpdateIfNeeded` returns). Wrappers keep calling `Stop` from Sleep and Close only.
- Do not add `context` cancellation to `HTTPGet`. Join-without-cancel meets Desired: after GET, `tick` skips `onUpdate`, join returns, generation stays disposed. Sleep/Close for that key may block up to `HTTPGetTimeout`.
- BIN ignore-after-close: `atomic.Bool` (or equivalent) set true before `Stop` join. `hotSwap` re-checks before assigning `w.db`. A copy opened after close is closed and removed. Do not add `sync.RWMutex` around `LookupRecord` / `hotSwap` publish.
- MMDB ignore-after-close: dedicated closed flag under the existing `mu`. Do not treat `db == nil` plus empty path as closed.
- Wake after Sleep: table protocol already waits for Sleep to return (slotBusy). Join makes that wait cover the GET. `startUpdate` may replace `w.updater` with a new `*Updater` after the previous `Stop`. No second join owner.
- Product tests live next to `pkg/dbwrappers` (and updater if needed) under ordinary `_test.go` names for delayed-download-until-after-close on BIN and MMDB. Not `zzz_proof_*`.
- Usage packets `core_geoblock_database_wrapper.md` / `core_geoblock_database_source.md` are enough to call the subsystem today. After apply, those usage lines that say Sleep/Close “stop the ticker” need join wording — later phase, not this explore write.
- Spec `core_geoblock_database_wrapper-reclaim` later: Close/Sleep join wording. Out of explore implement.

## Open questions

- Q: Can BIN ignore-after-close be a closed flag without F-1’s `sync.RWMutex` on `LookupRecord`?
  Rank: additive asked — new closed flag on BIN this change adds; Desired “BIN and MMDB ignore an update that arrives after close” and “Do not take F-1’s BIN LookupRecord mutex unless explore proves the ignore-after-close check cannot be made safe without it”
  Decision: assumed — `atomic.Bool` is enough. Set it true before `Stop` join; `hotSwap` re-checks immediately before publishing. Overlap with Close is closed by join-then-dispose. The LookupRecord vs handle race stays on `2026-09-14-bin-handle-race`.
  By: propose

- Q: Should `Stop` join wait out an in-flight GET (up to `HTTPGetTimeout`), or must this change cancel `HTTPGet`?
  Rank: bounded asked — `Updater.Stop` already has callers and this run counted them: 4 sites (`pkg/dbwrappers/bin.go` sleep+close, `pkg/dbwrappers/mmdb.go` sleep+close; searched `pkg/**` for `Updater.Stop` and `dbsource.Start`). Desired “Updater.Stop is synchronous: join the ticker goroutine”. Out of scope: “Adding context cancellation to HTTPGet unless explore shows join-without-cancel cannot meet Desired”
  Decision: assumed — join waits out the GET. Do not add `context` to `HTTPGet`. Measured: join-without-cancel meets Desired because `tick` can skip `onUpdate` after stop; wrappers still ignore a missed join. Reclaim Sleep keeps that key `slotBusy` until the GET returns; other keys are not under `t.mu` during the hook (`vendor/.../reclaim/table.go` runHook outside the table lock).
  By: propose

- Q: Can MMDB treat post-close `db == nil` plus empty path as closed, or does it need its own closed flag?
  Rank: additive asked — new closed flag on MMDB this change adds; Desired ignore-after-close; Unknowns “first-open path cannot be confused with dispose”
  Decision: assumed — own closed flag under the existing MMDB `mu`. Zero value is also pre-first-`open`; that pair cannot mean disposed.
  By: propose
