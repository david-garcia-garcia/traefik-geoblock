## Context

See proposal.md — Why. Two constraints shape the approach.

The first is the existing spec rule that log lines MUST NOT be emitted while the table mutex is held (`openspec/specs/std_go_reclaim_context-lease/spec.md`). That rules out the obvious ordering fix for `reclaim_orphan`: moving the log above `time.AfterFunc` inside the critical section in `drop`.

The second is Yaegi. `pkg/reclaim` is loaded by the Traefik plugin interpreter, so it stays non-generic, stdlib-only, and `create` keeps its zero-argument signature. Nothing here may add a dependency or a type parameter.

The current end-of-incarnation path is: `fire` (or `Reset`) cancels the lifetime, and a goroutine started by `Open` — `go func(){ waitCtx(life); stopValue(value) }()` — wakes up and calls `Close()`. That goroutine exists only to translate "lifetime canceled" into "Close the value". `life` is a `context.WithCancel(context.Background())` whose cancel function is reachable only from `fire` and `Reset`, so nothing outside the table can end that lifetime.

## Goals / Non-Goals

**Goals:**
- `reclaim_dispose` is a completion signal, not a hint: when it is emitted, `Close()` has returned.
- `reclaim_orphan` precedes `reclaim_dispose` for one incarnation at any grace duration, still without logging under the mutex.
- Tests prove the invariants that must hold on both sides of the grace edge, and never depend on which side of it a timer landed.
- The coverage gaps named in the proposal are closed.

**Non-Goals:**
- No change to `Open`'s signature, the five message constants, `DefaultGrace`, or how `pkg/dbwrappers` builds keys.
- No `-race` flag in CI and no test-only hook or fake clock injected into `Table`. The fix must hold for the production timer.
- The holder `watch` goroutine for a `context.Background()` holder still parks forever; that is a Yaegi accommodation, not this change.

## Decisions

### Close the value synchronously and delete the per-slot goroutine

`fire` and `Reset` call `stopValue(value)` immediately after `cancel()` and before the `reclaim_dispose` log. The `go func(){ waitCtx(life); stopValue(value) }()` in `Open` goes away.

Why over the alternative: upstream `traefik-modsecurity` fixed the same failure by adding a `waitUntil` in the test. That leaves the contract ambiguous — every future caller and every future test has to know that the dispose line does not mean the value is closed — and it costs a goroutine per incarnation forever. Closing inline makes the observable order deterministic, removes the goroutine, and matches the lost-create-race path at `table.go:140`, which already calls `stopValue` synchronously.

Considered and rejected: keeping the goroutine but having `fire` wait for it (a channel closed after `stopValue`). Same determinism, strictly more machinery, and the goroutine still exists.

Cost accepted: a value whose `Close()` blocks now blocks the grace-timer goroutine or the `Reset` caller. Values on this table close a ticker and a file handle. The spec states the constraint so a future caller cannot claim surprise.

### Order the orphan log with the generation counter, not the mutex

`drop` sets `e.arming = true` under the lock (alongside the `graceGen++` it already does), releases the lock, logs `reclaim_orphan`, then re-acquires the lock and arms `time.AfterFunc` only if the slot is still the mapped one and `graceGen` is unchanged. `bindLocked` treats `arming` exactly as it treats an armed timer: it is a reclaim, and it bumps `graceGen`.

Why this works: grace cannot start before the orphan line is emitted, and `fire` can only run from a timer armed after that line, so `reclaim_orphan` → `reclaim_dispose` is guaranteed. An `Open` that lands in the arming window bumps the generation, so the arm is skipped, the slot stays live, and `reclaim_reclaim` is logged after `reclaim_orphan` — the same sequence the spec already describes.

Why over the alternatives: logging inside the critical section is forbidden by the spec. Re-checking only `t.items[key] == e` without the generation would re-arm a slot that a concurrent `Open` had already reclaimed, which would drop `reclaim_reclaim` and leave a timer racing a live holder.

The zero-grace path is already ordered — it logs orphan and then calls `fire` on the same goroutine — and keeps that shape.

### Race-stress tests assert invariants, not a winner

`TestTable_ReclaimRacesFire` keeps its stress loop but adopts the shape its sibling `TestTable_ZeroGraceOpenRacesCancel` already uses: branch on which outcome occurred, then assert what must be true in that outcome. Same pointer means the incarnation must still be alive while a holder is live; a different pointer means the grace timer won and the old value must have been closed. The "a late `fire` must not dispose a live incarnation" invariant stays covered deterministically by `TestTable_StaleFireAfterReclaimNoops`, which calls `fire` directly with a stale generation and needs no timing at all.

Tests that assert the reclaim *branch* rather than the race (`TestTable_OpenDuringGraceReclaims`) get a grace long enough that the branch is certain. The rule for this package: a test either uses a grace long enough that the intended branch cannot lose, or it accepts both outcomes. No test may need a specific timer to win.

### Test-only hardening in `pkg/dbwrappers`

`useShortLeases` grows a grace argument so the two `SameHashReclaimKeepsTicker` tests get a lease they cannot lose, while the two `HashChangeDisposesOld` tests keep a short lease and poll for `reclaim_dispose` instead of sleeping a fixed 80 ms. No production file in `pkg/dbwrappers` changes.

## Risks / Trade-offs

- A stored value with a slow or blocking `Close()` now stalls the grace timer goroutine (or `Reset`) → the spec forbids blocking in `Close()` for values on this table, and both values in this product (`*BIN`, `*MMDB`) stop a ticker and close a file.
- `drop` takes the table mutex twice per orphan instead of once → orphan happens once per key per config reload; the extra acquisition is not on any request path.
- The arming window is a new state that `bindLocked`, `drop`, `fire`, and `Reset` must all agree on → covered by a direct-call test that binds inside the window, plus the existing stale-`fire` tests, plus the stress loops.
- Removing the per-slot goroutine changes when `Close()` runs relative to the value's own lifetime-watching goroutines → it does not: `cancel()` still happens first, exactly as before; only the caller of `Close()` moves.
- The remaining flake risk is a `waitUntil` budget exhausted on a very slow runner → the budget is a guard, not an assertion, and it is raised.
