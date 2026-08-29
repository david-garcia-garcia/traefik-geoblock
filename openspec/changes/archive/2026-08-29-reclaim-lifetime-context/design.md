## Context

See proposal.md — Why. `pkg/reclaim.Open` today is `create func() (any, error)` plus `dispose func(any)`. `fire`, `Reset`, and a lost create race invoke dispose. BIN/MMDB pass `close`; Plugin passes an empty func. Yaegi still forbids `Table[T]`.

## Goals / Non-Goals

**Goals:**
- One lifetime `context.Context` per incarnation, canceled at the same three sites as dispose.
- `create` receives that context so wrappers can stop work without a house callback.
- Existing reclaim log constants, including `reclaim_dispose`.

**Non-Goals:**
- Returning the lifetime from `Open`, or `Table.Context(key)`.
- Rewiring `dbsource.Updater` onto the lifetime (it keeps `stop chan`).
- Renaming spec folders or log message strings.
- Changing grace or the Traefik `New` holder context.

## Decisions

1. **Create takes `life`.**
   `Open(ctx, key, create func(life context.Context) (any, error)) (any, error)`.
   Alternative: return `life` from Open. Rejected — teardown belongs in create; a later Open does not re-run create.

2. **Table owns `WithCancel`.**
   Allocate `life, cancel` before `create`. Store `cancel` on the slot when this create wins. Call `cancel` in `fire`, `Reset`, and when this create loses the first-put race. Alternative: keep dispose and also cancel a context. Rejected — ticket drops dispose.

3. **Wrappers watch `life` in create.**
   `go func() { <-life.Done(); w.close() }()`. `close` still stops the Updater and the file. Alternative: pass `life` into `Updater.Start`. Rejected — out of scope.

4. **Keep `reclaim_dispose`.**
   The line means the incarnation ended. Tests that flipped a dispose flag instead watch `life.Done()` (table) or `close` / lookups (wrappers).

## Risks / Trade-offs

- [Lost-create race leaves an open file if create does not watch `life`] → wrappers MUST start the watch before returning; table cancels immediately on a lost race.
- [Create that ignores `life` leaks until GC] → Plugin has no file/loop on the incarnation; wrappers must watch.
- [Async `close` after `fire`] → same as today’s dispose-after-unlock; tests already wait past grace.

## Migration Plan

In-repo only. One apply: signature, three call sites, tests. Rollback is revert the change branch.
