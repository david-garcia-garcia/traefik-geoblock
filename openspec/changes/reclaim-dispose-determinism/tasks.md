## 1. Component: end an incarnation deterministically

- [ ] 1.1 In `pkg/reclaim/table.go`, call `stopValue(value)` in `fire` after `cancel()` and before the `reclaim_dispose` log
- [ ] 1.2 Do the same in `Reset` for every slot it takes off the map
- [ ] 1.3 Delete the per-slot `go func(){ waitCtx(life); stopValue(...) }()` in `Open` and drop the now-unused `life` variable name in favour of the cancel it already keeps on the slot
- [ ] 1.4 Confirm `waitCtx` is still needed for holder contexts (nil `Done`) and leave it in place

## 2. Component: order the orphan log before grace starts

- [ ] 2.1 Add an `arming bool` field to `slot` with a comment that says what window it names
- [ ] 2.2 In `drop`, set `arming` under the lock, unlock, log `reclaim_orphan`, then re-lock and arm `time.AfterFunc` only when the slot is still mapped and `graceGen` is unchanged; clear `arming` on both paths
- [ ] 2.3 Extend `drop`'s early return so a slot that is arming is treated like a slot with an armed timer
- [ ] 2.4 In `bindLocked`, treat `arming` as a reclaim: stop nothing, bump `graceGen`, report `reclaimed = true`
- [ ] 2.5 Keep the zero-grace path logging orphan and then calling `fire` on the same goroutine
- [ ] 2.6 Clear `arming` in `Reset` so a slot taken off the map cannot arm afterwards

## 3. Upstream parity

- [ ] 3.1 Rename the local variables in `Open` and `stopValue` to match `traefik-modsecurity` (`v` → `stored` / `created` / `value`)

## 4. Tests: remove the unwinnable races

- [ ] 4.1 Rewrite `TestTable_ReclaimRacesFire` to branch on the outcome (reclaimed vs new incarnation) and assert the invariant for that outcome, in the shape `TestTable_ZeroGraceOpenRacesCancel` already uses
- [ ] 4.2 Give `TestTable_OpenDuringGraceReclaims` a grace long enough that the reclaim branch cannot lose, and bound the "must not dispose" check instead of racing it
- [ ] 4.3 Raise `waitBudget` so a loaded runner cannot exhaust the guard
- [ ] 4.4 Keep the immediate (unwaited) read of the `Close()` side effect in `TestTable_HashChangeProof` — with task 1 it is now the contract test for dispose-implies-closed

## 5. Tests: prove the new contract

- [ ] 5.1 Assert `Close()` has returned when `reclaim_dispose` is observed, for both the grace path and `Reset`
- [ ] 5.2 Assert the recorded order is `reclaim_orphan` then `reclaim_dispose` at a sub-millisecond grace
- [ ] 5.3 Assert an `Open` that lands in the arming window reclaims the slot and logs `reclaim_reclaim` after `reclaim_orphan`
- [ ] 5.4 Assert the table's goroutine count returns to its pre-`Open` level after every incarnation ends

## 6. Tests: close the coverage gaps

- [ ] 6.1 Cover the `waitCtx` polling branch with a holder context whose `Done()` is nil but whose `Err()` becomes non-nil
- [ ] 6.2 Cover a stored value without `Close()` (no panic, dispose still logged)
- [ ] 6.3 Cover `ResetWith` actually applying the new grace to the process table, and `Reset` restoring `DefaultGrace`
- [ ] 6.4 Cover `Default()` under concurrent first use returning one table
- [ ] 6.5 Cover repeated orphan → reclaim → orphan → dispose cycles on one key (value identity kept, one dispose at the end)
- [ ] 6.6 Cover holder-map cleanup: many sequential opens on one key leave no holder entries behind
- [ ] 6.7 Cover the logger-ownership rule: orphan and dispose go to the last binding `Open`'s logger
- [ ] 6.8 Cover key independence at scale (many keys, one disposed, the rest untouched)
- [ ] 6.9 Cover `Reset` racing an in-flight `Open` (no panic, no value stored on a canceled lifetime)

## 7. Integration tests in `pkg/dbwrappers`

- [ ] 7.1 Give `useShortLeases` a grace argument; use a lease the reclaim tests cannot lose in `TestOpenBIN_SameHashReclaimKeepsTicker` and `TestOpenMMDB_SameHashReclaimKeepsTicker`
- [ ] 7.2 Replace the fixed 80 ms sleeps in `TestOpenBIN_HashChangeDisposesOld` and `TestOpenMMDB_HashChangeDisposesOld` with a poll for `reclaim_dispose` on the old key

## 8. Docs and verification

- [ ] 8.1 Update `knowledge/devdocs/std_go_reclaim.md`: dispose implies Close has returned, no goroutine per incarnation, `Close()` must not block, and the grace-edge rule for tests
- [ ] 8.2 `go build ./...`, `go vet ./...`, `golangci-lint` as configured, and `go test ./...`
- [ ] 8.3 Re-run the reproduction: `go test ./pkg/reclaim/ -count=25` under CPU contention, and confirm no failures
- [ ] 8.4 `openspec validate --strict reclaim-dispose-determinism`
