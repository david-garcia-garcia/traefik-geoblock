# Dead

1. [judgement] Dead branch — `pkg/dbsource/updater.go:87` — the restart capability this diff added to `Updater` has no production caller: no code path ever calls `Start` twice on one `*Updater`, so the already-running guard is never taken, and the `u.ticker, u.stop = nil, nil` reset in `Stop` (`updater.go:150`) exists only so a stopped `Updater` can be started again.

   ```go
   // Start runs an immediate check and a 24h ticker. onUpdate is called with a new path.
   // A second Start while the loop is running is ignored; after Stop it starts a fresh loop.
   func (u *Updater) Start(onUpdate func(path string)) {
   	if !u.CanDownload() {
   		return
   	}
   	u.mu.Lock()
   	if u.stop != nil {
   		u.mu.Unlock()
   		return
   	}
   ```

   Searched: `rg -n "updater\.|\.Stop\(\)|dbsource\.Start"` and `rg -n "\.Start\(|Updater\b"` over the tree excluding `vendor/`, `openspec/`, `devstate/`, `.cursor/`. The only production construction site is `dbsource.Start` (`updater.go:38`), which calls `u.Start` once on an `*Updater` it just built by `newUpdater`. `Stop` has exactly two production callers, `BIN.Sleep` (`pkg/dbwrappers/bin.go:343`) and `MMDB.Sleep` (`pkg/dbwrappers/mmdb.go:180`); both take the pointer and set `w.updater = nil` under `w.mu` before calling, so neither can Stop the same instance twice. `BIN.Wake` and `MMDB.Wake` do not restart the stopped loop — they call `startUpdate`, which goes back through `dbsource.Start` and builds a **fresh** `*Updater`. The only callers that reuse an instance are `pkg/dbsource/updater_test.go:191` (`TestUpdater_StopThenStartRunsAFreshLoop`, which also asserts the second-Start guard) and `updater_test.go:213` (`TestUpdater_StoppingTwiceIsSafe`).

   Marked judgement, not hard: `Updater`, `Start`, and `Stop` are exported on an exported type, and a deletion here is not a safe unattended apply — dropping the guard turns any future double `Start` into a leaked ticker goroutine. Record the contract mismatch instead of removing the code.
   → Either make `Wake` reuse the stored `*Updater` (`w.updater.Start(w.onUpdate)`) so the restart path this diff documents and tests is the production path, or drop the restart claim from the `Updater` and `Start` doc comments, delete the `u.stop != nil` guard together with the ticker/stop reset in `Stop`, and retarget `TestUpdater_StopThenStartRunsAFreshLoop` at the sleep/wake cycle in `pkg/dbwrappers/reclaim_test.go`, which is what production actually does.
   Status: open
   Argument: none.

## Checked and clean

- `pkg/reclaim/table.go`: no leftovers of the removed mechanisms. `rg -n "stopValue|valueClosed|arming|graceGen|graceTimer|nextID|bindLocked|logBind|holders\["` returns hits only in `openspec/` (the design and task notes that describe the removal) and in `*_test.go`. The per-slot lifetime context and its `cancel`, the parked `waitCtx(life)` goroutine, and the holder id map are gone from the package with no residue.
- All four `slotState` values are read on a production path. `slotBusy`, `slotAwake`, `slotAsleep` are obvious; `slotGone` is read by `drop` (`table.go:292`) and `expire` (`table.go:338`). The `case slotGone` in `Open` (`table.go:205`) is also reachable rather than defensive-only: `Reset` swaps `t.items` before it marks slots gone, so a slot whose `create` was in flight can be re-registered by `put` (`table.go:236`) into the new map and then marked `slotGone` by the `Reset` loop, leaving a mapped gone slot for the next `Open` to reclaim the key from. Not a finding.
- `slot.err` (`table.go:72`) is written only on create failure but read on a production path: a second concurrent first `Open` parks in the `slotBusy` case and returns that error (`table.go:200`).
- New reclaim helpers all have production callers: `sleeper`/`waker` are asserted by `sleepValue`/`wakeValue`, `dispose` is called by `expire` and `Reset`, and `BIN`/`MMDB` `Sleep`/`Wake`/`Close` are dispatched through those assertions from `Open`, `drop`, `expire`, and `Reset`.
- `pkg/dbwrappers`: the diff deleted the old unexported `close()` on both wrappers and left no callers behind — `rg -n "close\(\)"` finds no `w.close()` in the tree. The new `BIN.onUpdate` and `MMDB.onUpdate` are the callbacks passed to `dbsource.Start` from `startUpdate`, which both `newBIN`/`newMMDB` and `Wake` call.
- `pkg/dbsource`: the new `stopped` helper is read twice inside `tick`; `mu` and `running` are read by `Start` and `Stop`.
- `BIN.currentLocalDbCopy` (`bin.go:53`) is written by `createLocalCopy` and `hotSwap` and never read, but it was already write-only on `origin/master` (same two writes, no reader) — this diff only moved the `hotSwap` write under `w.mu`. Out of scope for the same-change rule.
