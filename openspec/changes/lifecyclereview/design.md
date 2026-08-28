## Context

See proposal.md Why. Measured on `traefik:v3.7.11`: `CreateRouters` cancels one factory context, then `New` again (~1 ms). Vendor `Close` returns nil. BIN hot-swap 10s delay is a different race (unsynchronized `Lookup`); leave it.

Yaegi v0.16.1: `*reclaim.Table[*BIN]` named in another package panics (`nodeType2`). Alias and embed of that type panic too. A non-generic table of `any` in another package plus a type-assert in the caller loads and runs.

## Goals / Non-Goals

**Goals:**

- Reusable lease+storage `Table` in `pkg/reclaim` (stdlib, stores `any`). Process singleton (`Default` / `Open`).
- Wrappers bind `New` ctx; unreclaimed hash disposed after 10s grace.
- Callers type-assert. Keys for BIN and MMDB are prefixed on the one table.

**Non-Goals:**

- Drop BIN hot-swap delayed close.
- OOTB LITE defaults or README.
- Cross-package generic `Table[*T]` (Yaegi).

## Decisions

1. **`pkg/reclaim.Table` stores `any`.**
   `NewTable(grace, logger)`, `Open(ctx, key, create, dispose)`. First `Open` creates and stores the value. Later `Open` (live or grace) returns the stored value and binds ctx. Last ctx Done starts grace; fire runs dispose once.
   Alternative: `Table[T]` in `pkg/dbwrappers` — rejected; not reusable across packages.
   Alternative: `pkg/reclaim.Table[T]` imported as `Table[*BIN]` — rejected; Yaegi panic.

2. **One process table.** `reclaim.Default()` / `reclaim.Open`. `OpenBIN` / `OpenMMDB` prefix keys (`bin:`, `mmdb:`) and assert. `InstanceLock` + `binByKey` stay gone.

3. **Set of contexts, not a refcount integer alone.**
   Each `Open` watches that `ctx.Done()`. Two routers, same hash, same generation: two binds, one cancel parent, grace starts once.

4. **Pass `ctx` down `New` → provider `New` → `OpenBIN` / `OpenMMDB`.**

5. **Default grace 10s.** Tests use a short grace.

6. **Usage packet `std_go_reclaim.md`:** import `pkg/reclaim`; assert in the owner package.

7. **Lifecycle logs.** Same `reclaim_*` msg constants + `key`. Info: put, dispose. Debug: bind, orphan, reclaim.

## Risks / Trade-offs

- [Slow Traefik apply > grace] → dispose then recreate. Mitigation: 10s vs ~1 ms measured.
- [Wrong assert] → `Open*` fails closed if the stored value is not `*BIN` / `*MMDB`.
- [Yaegi + extra goroutine per Open] → one watch per `New`. Acceptable.

## Migration Plan

- No public Traefik YAML change.
- Roll forward: new binary; old tickers die with the process.
- Rollback: previous binary leaks hash-change tickers again.

## Open Questions

None. `any` table in `pkg/reclaim`. Grace 10s stays.
