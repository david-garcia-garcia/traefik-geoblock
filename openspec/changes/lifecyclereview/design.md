## Context

See proposal.md Why. Measured on `traefik:v3.7.11`: `CreateRouters` cancels one factory context, then `New` again (~1 ms). Vendor `Close` returns nil. BIN hot-swap 10s delay is a different race (unsynchronized `Lookup`); leave it.

Yaegi v0.16.1: `*reclaim.Table[*BIN]` named in another package panics (`nodeType2`). Alias and embed of that type panic too. Same-package `Table[T]` + `var bins *Table[*BIN]` loads. `map[string]any` across packages also loads; we still store `T`.

## Goals / Non-Goals

**Goals:**

- Generic lease+storage `Table[T]` defined in the package that owns `T`.
- Wrappers bind `New` ctx; unreclaimed hash disposed after 10s grace.
- Copy `table.go` into another package to reuse.

**Non-Goals:**

- Drop BIN hot-swap delayed close.
- OOTB LITE defaults or README.
- A separate Go module. Cross-package `Table[*T]` (Yaegi).

## Decisions

1. **`Table[T]` in `pkg/dbwrappers`.**
   `NewTable[T](grace, logger)`, `Open(ctx, key, create, dispose)`. First `Open` creates and stores `T`. Later `Open` (live or grace) returns the stored value and binds ctx. Last ctx Done starts grace; fire runs dispose once.
   Alternative: `pkg/reclaim.Set` + caller maps — rejected; two owners.
   Alternative: `pkg/reclaim.Table[T]` imported by dbwrappers — rejected; Yaegi panic.
   Alternative: alias/embed of `reclaim.Table[*BIN]` — rejected; same panic.

2. **`var bins *Table[*BIN]` and `var mmdbs *Table[*MMDB]`.** Same package as the type arguments. `InstanceLock` + `binByKey` go away.
   Alternative: `any` map in another package — works on this host; rejected so the table stays typed.

3. **Set of contexts, not a refcount integer alone.**
   Each `Open` watches that `ctx.Done()`. Two routers, same hash, same generation: two binds, one cancel parent, grace starts once.

4. **Pass `ctx` down `New` → provider `New` → `OpenBIN` / `OpenMMDB`.**

5. **Default grace 10s.** Tests use a short grace.

6. **Usage packet `std_go_reclaim.md`:** copy `table.go` into the package that owns `T`.

7. **Lifecycle logs.** Same `reclaim_*` msg constants + `key`. Info: put, dispose. Debug: bind, orphan, reclaim.

## Risks / Trade-offs

- [Slow Traefik apply > grace] → dispose then recreate. Mitigation: 10s vs ~1 ms measured.
- [Copy drift] → accepted. The file must live next to `T`.
- [Yaegi + extra goroutine per Open] → one watch per `New`. Acceptable.

## Migration Plan

- No public Traefik YAML change.
- Roll forward: new binary; old tickers die with the process.
- Rollback: previous binary leaks hash-change tickers again.

## Open Questions

None. `Table[T]` in the owner package. Grace 10s stays.
