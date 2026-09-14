## Why

`Updater.Stop` does not join a tick already inside HTTP download. That tick still calls `onUpdate`, so BIN `hotSwap` and MMDB `open` publish a new handle on a generation reclaim has already disposed. Lookups succeed after Close; BIN also leaves a new temp copy in `os.TempDir()`.

## What Changes

- `Updater.Stop` joins the ticker goroutine. `tick` does not call `onUpdate` after stop.
- Sleep and Close keep calling that `Stop` only. One join owner.
- BIN ignore-after-close: `atomic.Bool` set true before the `Stop` join; `hotSwap` re-checks before publishing. A copy opened after close is closed and removed. No `sync.RWMutex` on `LookupRecord`.
- MMDB ignore-after-close: own closed flag under the existing `mu`. Do not treat `db == nil` plus empty path as closed.
- Do not add `context` to `HTTPGet`. Join waits out an in-flight GET (up to `HTTPGetTimeout`).
- Product tests for delayed-download-until-after-close on BIN and MMDB. Not `zzz_proof_*`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_database_wrapper-reclaim`: Close and Sleep join the keep-current loop. A disposed generation stays disposed: no `onUpdate` after stop; BIN and MMDB ignore an update that arrives after close.

## Impact

- `pkg/dbsource/updater.go` (`Start`, `tick`, `Stop`)
- `pkg/dbwrappers/bin.go` (`hotSwap`, `close`)
- `pkg/dbwrappers/mmdb.go` (`open`, `close`)
- Product tests next to `pkg/dbwrappers` (and updater if needed)
- Usage packets `core_geoblock_database_wrapper.md` / `core_geoblock_database_source.md`: Sleep/Close “stop the ticker” join wording after apply (`opd-devdocsimpact`)
