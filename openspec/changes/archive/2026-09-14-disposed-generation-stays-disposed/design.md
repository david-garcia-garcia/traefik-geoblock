## Context

See proposal.md. `pkg/dbsource/updater.go` `Stop` closes `stop` and does not wait. `tick` always calls `onUpdate` after `UpdateIfNeeded`. `HTTPGet` has no `context` (`HTTPGetTimeout` = 30m). BIN `close` / `hotSwap` have no closed flag; `db == nil` is also AllowMissing-before-first-file. MMDB `close` clears the reader under `mu`; `open` has no closed check. Join owner is Updater (`explore.md` Decisions).

## Goals / Non-Goals

**Goals:**
- `Updater.Stop` waits for the ticker goroutine (`done` channel).
- `tick` reads `stop` after `UpdateIfNeeded` and skips `onUpdate`.
- BIN `atomic.Bool` closed flag set true before `Stop`; `hotSwap` re-checks before assign; leftover copy closed and removed.
- MMDB closed flag under existing `mu`; `open` refuses publish after close.
- Product tests delayed-download-until-after-close for BIN and MMDB (`pkg/dbwrappers`, ordinary `_test.go`).

**Non-Goals:**
- `context` on `HTTPGet`.
- F-1 `sync.RWMutex` on BIN `LookupRecord` / `hotSwap` (`2026-09-14-bin-handle-race`).
- `zzz_proof_*` filenames.
- Rewording usage packets in this apply (later `opd-devdocsimpact`).

## Decisions

1. **Join lives on `Updater.Stop`** — Wrappers already call `Stop` from Sleep and Close. A second wait in `close` would split the owner. Alternative: wrappers `Wait` after `Stop` — rejected; One job, one owner.

2. **Skip `onUpdate` after stop, do not cancel GET** — After the download returns, `tick` selects on `stop` and returns. Join-without-cancel meets Desired. Alternative: `HTTPGet` `context` — rejected; out of scope while join meets Desired. Sleep/Close for that reclaim key may block up to `HTTPGetTimeout`; other keys stay usable (`runHook` is outside `t.mu`).

3. **BIN closed flag is `atomic.Bool`** — Set true before `Stop` join. `hotSwap` loads it immediately before assigning `w.db`. Overlap with Close is closed by join-then-dispose. Alternative: F-1 `sync.RWMutex` around Lookup — rejected; ignore-after-close does not need it. Alternative: treat `db == nil` as closed — rejected; AllowMissing starts nil.

4. **MMDB closed flag under existing `mu`** — Zero value is also pre-first-`open`. Alternative: `db == nil` plus empty path — rejected; that pair is not disposed.

5. **Wake after Sleep** — Table protocol waits for Sleep to return (`slotBusy`). Join covers the GET. `startUpdate` may replace `w.updater` with a new `*Updater` after the previous `Stop`. No second join owner.

6. **Tests next to wrappers** — Hold the GET (httptest gate) until after Close, then release. Assert Lookup fails and BIN temp copy is gone. Names like `TestOpenBIN_DelayedDownloadAfterClose` / `TestOpenMMDB_DelayedDownloadAfterClose`. An updater unit test may exist if `tick` skip is easier there; the product contract is the wrapper tests.

## Risks / Trade-offs

- [Sleep/Close block up to 30m on an in-flight GET] → Mitigation: accepted. Reclaim parks that key only. Do not add HTTP cancel in this change.
- [Implement adds BIN Lookup mutex while touching `hotSwap`] → Mitigation: tasks forbid it; debt `knowledge/debt/2026-09-14-bin-lookuprecord-hotswap-unsynchronized.md`.
- [Missed join still republishes] → Mitigation: wrapper closed flags; BIN closes and removes a late copy.

## Migration Plan

Ship in the next plugin build. No config migration. Operators see the same Traefik `New` reuse; Close/Sleep for a URL-backed wrapper may wait out an in-flight download instead of returning immediately.

## Open Questions

Ticket questions live on `devstate/explore.md`.
