## Why

`pkg/reclaim` decides whether a database handle survives a Traefik config reload, and its `reclaim_dispose` log line is what everything else — tests, `pkg/dbwrappers`, an operator reading debug logs — treats as "this incarnation is over". Today that line is not a completion signal: `fire` cancels the lifetime and logs dispose while the value's `Close()` runs on a separate per-slot goroutine, so the log routinely wins the race. That is a reproduced CI failure (`TestTable_HashChangeProof`, `ended: []`), and a second reproduced failure comes from a test that demands the reclaim branch win a 3 ms timer race it cannot be guaranteed to win (`TestTable_ReclaimRacesFire`, 10 failures in 25 stressed runs). A third, narrower defect: `drop` arms the grace timer before it logs `reclaim_orphan`, so dispose can be logged before orphan for the same key.

## What Changes

- `reclaim_dispose` becomes a completion signal: after canceling the incarnation lifetime, `fire` and `Reset` wait for the lifetime goroutine to finish closing the value, and only then emit the dispose line. The lifetime context and its goroutine stay exactly as they are — this is an added handshake, not a removal.
- `reclaim_orphan` is always logged before the grace timer is armed, and therefore always before `reclaim_dispose` for that incarnation — without logging while the table mutex is held. `drop` marks the slot as arming under the lock, logs outside it, then arms only if the slot is still mapped and its grace generation has not moved; a bind that lands in that window reclaims the slot as it does today.
- The race-stress tests accept every legal outcome of the grace edge instead of requiring reclaim to win it, and the tests that assert the reclaim *branch* get a grace long enough that the branch is certain rather than probable.
- Upstream parity with `david-garcia-garcia/traefik-modsecurity` `pkg/reclaim`: the local-variable renames in `table.go` (`v` → `stored` / `created`).
- New coverage for surface no test reaches today: the nil-`Done` holder polling branch, `stopValue` on a value without `Close()`, `ResetWith` grace on the process table, `Default()` under concurrent first use, repeated orphan → reclaim → orphan → dispose cycles, holder-map cleanup, the logger-ownership rule for orphan and dispose, key independence at scale, `Reset` racing an in-flight `Open`, and a goroutine-leak guard.
- `pkg/dbwrappers/reclaim_test.go` stops depending on a 25 ms reclaim window and fixed 80 ms sleeps.
- Grace becomes a property of the incarnation instead of the table. `Open` takes a `grace`, the `Open` that creates the value fixes it, and `TableGrace` (any negative duration) means "take the table's". The table's grace stays as the default it always was, so one table can hold a plugin instance that must survive a reload next to a handle that should go the moment its holder does. An `Open` that reclaims cannot change the grace of the incarnation it found, the same way it cannot re-run `create`.

`Open` gains one argument. Message constants and the default grace are unchanged, and every existing caller keeps today's behavior by passing `TableGrace`.

## Capabilities

### New Capabilities
<!-- none: this change tightens an existing contract -->

### Modified Capabilities
- `std_go_reclaim_context-lease`: `reclaim_dispose` is emitted only after the stored value's `Close()` has returned; `reclaim_orphan` is emitted before grace starts, so it always precedes `reclaim_dispose` for the same incarnation; no goroutine the table starts for a key outlives that key; grace belongs to the incarnation, fixed by the `Open` that created it, with the table's grace as the default.

## Impact

- `pkg/reclaim/table.go` — `fire`, `Reset`, `drop`, `bindLocked`, `Open`; new `arming`, `valueClosed` and `grace` fields on `slot`; new `TableGrace` constant; `Open` takes a grace. Nothing is removed: this package is a copy shared with `traefik-modsecurity` and every edit must port back verbatim.
- `pkg/reclaim/default.go` — the process-table `Open` façade takes the same grace.
- Call sites pass `reclaim.TableGrace` (`plugin.go`, `pkg/dbwrappers/bin.go`, `pkg/dbwrappers/mmdb.go`), which is today's behavior.
- `pkg/reclaim/table_test.go` — race-stress assertions, grace values, new coverage.
- `pkg/dbwrappers/reclaim_test.go` — lease and wait hardening (tests only).
- `openspec/specs/std_go_reclaim_context-lease/spec.md` — three requirement edits.
- `knowledge/devdocs/std_go_reclaim.md` — the dispose-is-a-completion-signal rule and the goroutine note.
- Callers (`pkg/dbwrappers`, `plugin.go`) are unaffected at the API level; a value with a slow `Close()` now blocks the grace timer goroutine or `Reset` instead of a throwaway goroutine.
