# Requirement
IssueKey: 2026-09-08-reclaim-lifecycle

## Problem

A stored value in `pkg/reclaim` has two events, create and close. When the last holder context is
Done the incarnation stays *fully alive* for the whole grace period: a GeoIP wrapper keeps its
24h update ticker running and its file handle open for a resource nobody holds. The value needs
four events — `create -> (sleep -> wake)* -> sleep -> close` — so the idle window is cheap.

Second, the table itself carries plumbing that nothing outside the package can observe (a
per-slot `context.WithCancel` lifetime plus a goroutine parked on it) and a lost-create race that
duplicates a GeoIP download. Adding two more states on top of that shape makes it worse; the
table has to be restructured so the four states are simple to express.

## Current (code)

- `pkg/reclaim/table.go` (260 lines). `slot` has 7 fields: `value`, `cancel`, `holders`,
  `nextID`, `graceTimer`, `graceGen`, `logger`.
- `pkg/reclaim/table.go:124-159` — `Open` builds `life, cancel := context.WithCancel(...)`, runs
  `create()` **outside** `t.mu`, and starts `go func(){ waitCtx(life); stopValue(v) }()`. `life`
  is never handed to `create`, so no caller can observe it.
- `pkg/reclaim/table.go:132-144` — the lost-create branch: two first `Open`s for one key both run
  `create`; the loser calls `cancel()` + `stopValue(v)` and throws its value away.
- `pkg/reclaim/table.go:223-238` — `fire` deletes the key, calls `cancel()`, then logs
  `reclaim_dispose` **without waiting** for the parked goroutine to finish `Close()`. Dispose can
  therefore be emitted before `Close()` has returned.
- `pkg/reclaim/table.go:194-220` — `drop` arms the grace timer under `t.mu`, unlocks, then logs
  `reclaim_orphan`. At zero grace it unlocks, logs orphan, then calls `fire` inline.
- `pkg/reclaim/table.go:167-177` — `bindLocked` sets `reclaimed` whenever a grace timer was
  stopped; `logBind` then emits `reclaim_reclaim` before `reclaim_bind`.
- `pkg/reclaim/table.go:87-95` — `closer` is the only optional interface (`Close()`), asserted in
  `stopValue`.
- `pkg/reclaim/default.go` (44 lines) — process table, package `Open`, `Reset`, `ResetWith`.
- `pkg/reclaim/table_test.go` (896 lines, 26 tests) — behavior-level tests including
  `TestTable_StdlibImports`.
- `pkg/dbsource/updater.go:75-92` — `Start` creates `u.ticker` and `u.stop`, then `go func(){
  u.tick(onUpdate); for { select { case <-u.ticker.C: u.tick(...); case <-u.stop: return } } }()`.
  There is no `sync.WaitGroup` and no join.
- `pkg/dbsource/updater.go:94-104` — `tick` calls `UpdateIfNeeded` (network GET + file write)
  with **no check of `u.stop`** before or after the download.
- `pkg/dbsource/updater.go:106-121` — `Stop` stops the ticker and closes `u.stop`, then returns
  immediately. A `tick` already in flight keeps downloading and writing into `cfg.Dir` after
  `Stop` returned. `Start` is not restartable: it overwrites `u.ticker`/`u.stop` unconditionally.
- `pkg/dbwrappers/bin.go:311-324` — `BIN.Close()` calls `w.updater.Stop()` then `w.db.Close()`.
  No sleep/wake. `w.updater` is only set in `newBIN` (`startUpdate`).
- `pkg/dbwrappers/mmdb.go:158-173` — `MMDB.Close()` same shape, under `w.mu`.
- `pkg/dbwrappers/reset.go` — `Reset`/`ResetWith` delegate to `reclaim`.
- `openspec/specs/std_go_reclaim_context-lease/spec.md` — the current contract (7 requirements).
  On this branch's baseline (`origin/master`) it does **not** yet state the two hardening
  guarantees that exist on `2026-09-08-fix-reclaim`.
- `knowledge/devdocs/std_go_reclaim.md` — Language block defines Table, Default, Open, Grace,
  Lifetime. `Grace` is defined as "wait ... before the incarnation lifetime is canceled".

## Desired

1. Four-event lifecycle `create -> (sleep -> wake)* -> sleep -> close` on the stored value,
   exposed as **optional interfaces** next to today's `closer`. `create` keeps the signature
   `func() (any, error)`.
2. `sleep` runs when the last holder is Done; the value stays stored with the same pointer.
3. `wake` runs before `Open` returns a stored sleeping value. A caller never receives a sleeper.
   `wake` cannot fail (no error return, no create fallback).
4. `close` is ALWAYS preceded by `sleep`, on every ending path: grace expiry (already asleep —
   must not sleep twice), `Reset` on a live incarnation (sleep then close), and the loser of a
   create race if such a path still exists.
5. Zero grace: create, sleep, close back to back; the value is not kept.
6. Remove the unobservable `context.WithCancel` lifetime, its `cancel`, and the parked goroutine.
   Close the value directly; emit `reclaim_dispose` after `Close()` returns.
7. Remove the log-ordering flag (`arming` on the hardened branch; the same hazard exists on
   master as the unlock-then-log window) in favour of a structural ordering property.
8. Register the key **before** running `create`, so a second first-`Open` waits for the first
   result instead of duplicating a download. The lost-create branch disappears.
9. Decide, with evidence, whether the slot is owned by a per-key goroutine or by the table mutex.
10. Grace is redefined as: how long a **sleeping** value is kept before it is disposed. Spec and
    devdocs say so, and say the reason to keep grace long is "a sleeping value is cheap to keep".
11. `pkg/dbsource.Updater`: `Stop` joins its goroutine; `tick` does not download after stop;
    `Start` after `Stop` works deterministically.
12. `pkg/dbwrappers` BIN and MMDB implement sleep/wake (sleep stops+joins the updater and
    releases the handle; wake reopens and restarts the ticker).

## Affected

- `pkg/reclaim/table.go`, `pkg/reclaim/default.go`, `pkg/reclaim/table_test.go`
- `pkg/dbsource/updater.go` (+ tests)
- `pkg/dbwrappers/bin.go`, `pkg/dbwrappers/mmdb.go`, `pkg/dbwrappers/reclaim_test.go`
- `openspec/specs/std_go_reclaim_context-lease/spec.md`
- `knowledge/devdocs/std_go_reclaim.md`

## Out of scope

Listed, not taken:

- Porting the change back into `david-garcia-garcia/traefik-modsecurity` (the copy stays
  file-copyable, but the port is a separate act).
- Any `wake` error path or create-fallback (explicitly excluded by the human).
- Generic `Table[T]`, or changing `create`'s signature.
- Fixing `core_geoblock_database_token-download-file` spec validation (pre-existing, PR #60).
- Reworking BIN hot-swap or the 10s delayed `oldDB.Close()` in `bin.go:242-247`.

## Unknowns

- Whether a per-key owner goroutine keeps the lost-create race gone rather than moving it
  (owner must deregister before exiting; `Open` may grab a handle to a leaving goroutine).
- Whether an existing sleep/wake reclaim implementation already exists in another repo owned by
  `david-garcia-garcia` (search in flight).

## Tensions

- The ticket says the table is "about 320 lines with a nine-field slot". That describes
  `2026-09-08-fix-reclaim` (PR #82), not this branch's baseline. On `origin/master` it is 260
  lines with a 7-field slot, and there is no `arming` field — the log-ordering hazard on master
  is the plain unlock-then-log window. The redesign target is unchanged; the "remove a third"
  measurement must be stated against `origin/master`.
- The two guarantees the ticket says "must survive" (`reclaim_dispose` only after `Close()`
  returns; `reclaim_orphan` always precedes `reclaim_dispose`) are **not** guaranteed by
  `origin/master`: `fire` logs dispose immediately after `cancel()` without joining the closing
  goroutine. This change therefore *adds* them rather than preserving them.
- "Never remove functionality from this shared component" vs removing the per-slot lifetime. The
  human has ruled: the rule protects functionality, not unobservable plumbing.
