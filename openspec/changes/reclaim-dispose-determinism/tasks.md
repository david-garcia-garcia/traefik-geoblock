## 0. Ground rule

- [x] 0.1 `pkg/reclaim` is a copy shared with `traefik-modsecurity`: every edit is additive and self-contained, nothing existing is removed or reshaped, and the package keeps importing only the standard library

## 1. Component: end an incarnation deterministically

- [x] 1.1 Add a `valueClosed chan struct{}` to `slot`, made in `Open` and closed by the lifetime goroutine after `stopValue`
- [x] 1.2 In `fire`, wait for that channel after `cancel()` and before the `reclaim_dispose` log
- [x] 1.3 In `Reset`, cancel and wait the same way for every slot it takes off the map
- [x] 1.4 Keep the lifetime context, its cancel, the lifetime goroutine, and `waitCtx` exactly as they are

## 2. Component: order the orphan log before grace starts

- [x] 2.1 Add an `arming bool` field to `slot` with a comment that says what window it names
- [x] 2.2 In `drop`, set `arming` under the lock, unlock, log `reclaim_orphan`, then re-lock and arm `time.AfterFunc` only when the slot is still mapped and `graceGen` is unchanged; clear `arming` on both paths
- [x] 2.3 Extend `drop`'s early return so a slot that is arming is treated like a slot with an armed timer
- [x] 2.4 In `bindLocked`, treat `arming` as a reclaim: stop nothing, bump `graceGen`, report `reclaimed = true`
- [x] 2.5 Keep the zero-grace path logging orphan and then calling `fire` on the same goroutine
- [x] 2.6 Clear `arming` in `Reset` so a slot taken off the map cannot arm afterwards

## 3. Upstream parity

- [x] 3.1 Rename the local variables in `Open` and `stopValue` to match `traefik-modsecurity` (`v` → `stored` / `created` / `value`)

## 4. Tests: remove the unwinnable races

- [x] 4.1 Rewrite `TestTable_ReclaimRacesFire` to branch on the outcome (reclaimed vs new incarnation) and assert the invariant for that outcome, in the shape `TestTable_ZeroGraceOpenRacesCancel` already uses
- [x] 4.2 Give `TestTable_OpenDuringGraceReclaims` a grace long enough that the reclaim branch cannot lose, and bound the "must not dispose" check instead of racing it
- [x] 4.3 Raise `waitBudget` so a loaded runner cannot exhaust the guard
- [x] 4.4 Keep the immediate (unwaited) read of the `Close()` side effect in `TestTable_HashChangeProof` — with task 1 it is now the contract test for dispose-implies-closed
- [x] 4.5 Task 1 inverts the order of the two observables (`Close` flag now precedes the dispose line), so every test that waited on a flag and then read the log must wait for the line instead: `TestTable_OpenCancelDispose`, `TestTable_ZeroGraceEndsImmediately`, `TestTable_ConcurrentCancelLastHolders`
- [x] 4.6 Assert the flag after the line in those three, which turns each of them into a second check of dispose-implies-closed

## 5. Tests: prove the new contract

- [x] 5.1 Assert `Close()` has returned when `reclaim_dispose` is observed, for both the grace path and `Reset`
- [x] 5.2 Assert the recorded order is `reclaim_orphan` then `reclaim_dispose` at a sub-millisecond grace
- [x] 5.3 Assert an `Open` that lands in the arming window reclaims the slot (that bind is not ordered against the orphan line, so do not assert an order it cannot hold)
- [x] 5.5 Assert that a reclaim plus release inside the arming window still ends with grace armed, so the slot cannot be stranded mapped with no holders and no timer
- [x] 5.4 Assert the goroutine count returns to its pre-`Open` level after many keys end (leak guard for both the watcher and the lifetime goroutine)

## 6. Tests: close the coverage gaps

- [x] 6.1 Cover the `waitCtx` polling branch with a holder context whose `Done()` is nil but whose `Err()` becomes non-nil
- [x] 6.2 Cover a stored value without `Close()` (no panic, dispose still logged)
- [x] 6.3 Cover `ResetWith` actually applying the new grace to the process table, and `Reset` restoring `DefaultGrace`
- [x] 6.4 Cover `Default()` under concurrent first use returning one table
- [x] 6.5 Cover repeated orphan → reclaim → orphan → dispose cycles on one key (value identity kept, one dispose at the end)
- [x] 6.6 Cover holder-map cleanup: many sequential opens on one key leave no holder entries behind
- [x] 6.7 Cover the logger-ownership rule: orphan and dispose go to the last binding `Open`'s logger
- [x] 6.8 Cover key independence at scale (many keys, one disposed, the rest untouched)
- [x] 6.9 Cover `Reset` racing an in-flight `Open` (no panic, no value stored on a canceled lifetime)

## 7. Integration tests in `pkg/dbwrappers`

- [x] 7.1 Give `useShortLeases` a grace argument; use a lease the reclaim tests cannot lose in `TestOpenBIN_SameHashReclaimKeepsTicker` and `TestOpenMMDB_SameHashReclaimKeepsTicker`
- [x] 7.2 Replace the fixed 80 ms sleeps in `TestOpenBIN_HashChangeDisposesOld` and `TestOpenMMDB_HashChangeDisposesOld` with a poll for `reclaim_dispose` on the old key

## 8. Docs and verification

- [x] 8.1 Update `knowledge/devdocs/std_go_reclaim.md`: the shared-copy sync rule, dispose implies Close has returned, `Close()` must not block, and the grace-edge rule for tests
- [x] 8.2 `go build ./...`, `go vet ./...`, `golangci-lint` as configured, and `go test ./...`
- [x] 8.3 Re-run the reproduction: `go test ./pkg/reclaim/ -count=25` under CPU contention, and confirm no failures
- [x] 8.4 `openspec validate --strict reclaim-dispose-determinism`
- [x] 8.5 Prove the new tests catch the defects: build them against `origin/master`'s `table.go` in a scratch module and confirm they fail there
