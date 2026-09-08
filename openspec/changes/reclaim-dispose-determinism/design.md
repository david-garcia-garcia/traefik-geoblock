## Context

See proposal.md — Why. Three constraints shape the approach.

The first is that `pkg/reclaim` is a **shared copy**. The same package lives in `david-garcia-garcia/traefik-modsecurity`, and the two must stay in sync in both directions: whatever lands here gets ported there, and whatever lands there gets ported here. So this change is additive only — fix defects and add coverage, never remove or reshape existing functionality — and it must not depend on anything outside `pkg/reclaim` or on this repo's callers.

The second is the existing spec rule that log lines MUST NOT be emitted while the table mutex is held (`openspec/specs/std_go_reclaim_context-lease/spec.md`). That rules out the obvious ordering fix for `reclaim_orphan`: moving the log above `time.AfterFunc` inside the critical section in `drop`.

The third is Yaegi. `pkg/reclaim` is loaded by the Traefik plugin interpreter, so it stays non-generic, stdlib-only, and `create` keeps its zero-argument signature. Nothing here may add a dependency or a type parameter.

The current end-of-incarnation path is: `fire` (or `Reset`) cancels the lifetime, and a goroutine started by `Open` — `go func(){ waitCtx(life); stopValue(value) }()` — wakes up and calls `Close()`. Nothing makes the canceller wait for that goroutine, which is why the dispose line can be emitted first.

## Goals / Non-Goals

**Goals:**
- `reclaim_dispose` is a completion signal, not a hint: when it is emitted, `Close()` has returned.
- Every edit is additive, so the whole change ports to `traefik-modsecurity` as-is.
- `reclaim_orphan` precedes `reclaim_dispose` for one incarnation at any grace duration, still without logging under the mutex.
- Tests prove the invariants that must hold on both sides of the grace edge, and never depend on which side of it a timer landed.
- The coverage gaps named in the proposal are closed.

**Non-Goals:**
- No removal or reshaping of anything the package already does. The incarnation lifetime context, its cancel, and the goroutine that watches it all stay.
- No change to `Open`'s signature, the five message constants, `DefaultGrace`, or how `pkg/dbwrappers` builds keys.
- No `-race` flag in CI and no test-only hook or fake clock injected into `Table`. The fix must hold for the production timer.
- The holder `watch` goroutine for a `context.Background()` holder still parks forever; that is a Yaegi accommodation, not this change.

## Decisions

### Make the canceller wait for the lifetime goroutine

The slot gets a `closed chan struct{}`. The lifetime goroutine keeps doing exactly what it does today and then closes that channel:

```go
go func() {
    waitCtx(life)
    stopValue(created)
    close(e.closed)
}()
```

`fire` and `Reset` cancel the lifetime as they already do, then wait on that channel, and only then log `reclaim_dispose`. The wait is a helper (`waitClosed`) that no-ops on a slot without the channel, matching the existing `if cancel != nil` defensiveness.

Why over the alternative: upstream `traefik-modsecurity` fixed the same failure by adding a `waitUntil` in the test. That leaves the contract ambiguous — every future caller and every future test has to know that the dispose line does not mean the value is closed.

Considered and rejected: calling `stopValue` inline in `fire` / `Reset` and deleting the lifetime goroutine. It is a smaller file, but it removes existing functionality from a package that is a shared copy, and this change is additive by rule (see Context).

Cost accepted: a value whose `Close()` blocks now blocks the grace-timer goroutine or the `Reset` caller instead of a detached goroutine. Values on this table stop a ticker and close a file handle, and `Updater.Stop` only closes a channel. The spec states the constraint so a future caller cannot claim surprise.

### Order the orphan log with the generation counter, not the mutex

`drop` sets `e.arming = true` under the lock (alongside the `graceGen++` it already does), releases the lock, logs `reclaim_orphan`, then re-acquires the lock and arms `time.AfterFunc` only if the slot is still the mapped one and still has no holders. `bindLocked` treats `arming` exactly as it treats an armed timer: it is a reclaim, and it bumps `graceGen`.

Why this works: grace cannot start before the orphan line is emitted, and `fire` can only run from a timer armed after that line, so `reclaim_orphan` → `reclaim_dispose` is guaranteed for the dispose that ends grace. An `Open` that lands in the arming window is a reclaim and keeps the slot live. It is **not** ordered against the orphan line — the bind takes the mutex `drop` released in order to log, so the recorded order can be `reclaim, bind, orphan`. Only a bind that lands after the arm is ordered, because it must take the mutex after `drop`'s second critical section, which is sequenced after the log. That is the ordering the spec states, and it is the one an operator reads: a reclaim inside the arming window is a sub-millisecond window that no configuration reaches deliberately.

Why over the alternatives: logging inside the critical section is forbidden by the spec.

What the second critical section must **not** do is decide on the generation it logged for. An `Open` can reclaim the slot during the orphan line and its holder can then go away, and that holder's own `drop` returns early because `arming` is still set — so if this `drop` bails on the generation mismatch, nobody arms grace and the slot is stranded: mapped, no holders, no timer, `Close()` never called, `t.items` entry never evicted. So the re-lock checks the state it actually cares about (still mapped, still no holders) and arms against the generation it reads at that moment. `TestTable_ReclaimAndReleaseInsideOrphanWindowStillArms` drives exactly that interleaving with a log handler that blocks on the orphan line.

The zero-grace path is already ordered — it logs orphan and then calls `fire` on the same goroutine — and keeps that shape.

### Race-stress tests assert invariants, not a winner

`TestTable_ReclaimRacesFire` keeps its stress loop but adopts the shape its sibling `TestTable_ZeroGraceOpenRacesCancel` already uses: branch on which outcome occurred, then assert what must be true in that outcome. Same pointer means the incarnation must still be alive while a holder is live; a different pointer means the grace timer won and the old value must have been closed. The "a late `fire` must not dispose a live incarnation" invariant stays covered deterministically by `TestTable_StaleFireAfterReclaimNoops`, which calls `fire` directly with a stale generation and needs no timing at all.

Tests that assert the reclaim *branch* rather than the race (`TestTable_OpenDuringGraceReclaims`) get a grace long enough that the branch is certain. The rule for this package: a test either uses a grace long enough that the intended branch cannot lose, or it accepts both outcomes. No test may need a specific timer to win.

### Test-only hardening in `pkg/dbwrappers`

`useShortLeases` grows a grace argument so the two `SameHashReclaimKeepsTicker` tests get a lease they cannot lose, while the two `HashChangeDisposesOld` tests keep a short lease and poll for `reclaim_dispose` instead of sleeping a fixed 80 ms. No production file in `pkg/dbwrappers` changes.

## Risks / Trade-offs

- A stored value with a slow or blocking `Close()` now stalls the grace timer goroutine (or `Reset`) → the spec forbids blocking in `Close()` for values on this table, and both values in this product (`*BIN`, `*MMDB`) stop a ticker and close a file.
- The two copies of this package drift if one side lands a change and the other is not ported → every edit here is additive and self-contained, so the port is a file copy; the port itself is tracked as a follow-up on the card.
- `drop` takes the table mutex twice per orphan instead of once → orphan happens once per key per config reload; the extra acquisition is not on any request path.
- The arming window is a new state that `bindLocked`, `drop`, `fire`, and `Reset` must all agree on → covered by a direct-call test that binds inside the window, plus the existing stale-`fire` tests, plus the stress loops.
- A `Close()` that never returns would now hang `fire` or `Reset` forever instead of leaking a goroutine → the same value would previously have leaked its ticker and file handle silently; a hang is the louder failure, and the spec names the constraint.
- The remaining flake risk is a `waitUntil` budget exhausted on a very slow runner → the budget is a guard, not an assertion, and it is raised.
