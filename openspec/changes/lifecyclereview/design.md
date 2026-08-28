## Context

See proposal.md Why. Measured on `traefik:v3.7.11`: `CreateRouters` cancels one factory context, then `New` again (~1 ms). `OpenBIN` / `OpenMMDB` already singleton by `configHash` via `InstanceLock` + a typed map in `pkg/dbwrappers`. `InstanceLock` must not grow a `map[string]any` (Yaegi). Vendor `Close` returns nil. BIN hot-swap 10s delay is a different race (unsynchronized `Lookup`); leave it.

## Goals / Non-Goals

**Goals:**

- One copy-pasteable package: stdlib only, no `T`, dispose is `func()`.
- Wrappers bind `New` ctx; unreclaimed hash disposed after 10s grace.
- Same helper usable later for any `New`-started background work.

**Non-Goals:**

- Drop BIN hot-swap delayed close.
- OOTB LITE defaults or README.
- A separate Go module or vanity import. Reuse = copy `pkg/reclaim`.
- Generics (Yaegi).

## Decisions

1. **Package `pkg/reclaim`, type `Set`.**
   `NewSet(grace time.Duration) *Set`. `Bind(key string, ctx context.Context, dispose func())`. First bind for a key stores dispose. Last live ctx Done starts grace; `Bind` same key cancels grace. Grace fire runs dispose once and forgets the key.
   Alternative: generic `Registry[T]` — rejected; Yaegi + `any` map already failed in `dbprovider`.
   Alternative: put the timer inside `dbwrappers` only — rejected; human asked for other-project reuse.

2. **Caller keeps the typed singleton.**
   `dbwrappers` still owns `map[string]*BIN` / `*MMDB` and `InstanceLock`. After create-or-get, `reclaim.Bind(hash, ctx, func() { w.close(); delete(map, hash) })`. Reclaim never sees `*BIN`.
   Alternative: reclaim stores the wrapper — rejected (Yaegi `any`).

3. **Set of contexts, not a refcount integer alone.**
   Each `Bind` watches that `ctx.Done()`. Last watch to fire (and no remaining live ctx) starts grace. Two routers, same hash, same generation: two binds, one cancel parent, grace starts once.
   Alternative: one ctx per key — rejected; two `New` same hash.

4. **Pass `ctx` down `New` → provider `New` → `OpenBIN` / `OpenMMDB`.**
   Signature add, not a package-level context.

5. **Default grace 10s.** Overridable in `NewSet` for tests (short grace). Product wrappers use 10s. Not tied to BIN hot-swap 10s except as the same order of magnitude as the apply gap (~1 ms).

6. **Usage packet `std_go_reclaim.md`** (root `std`, domain `go`) so another repo can implement from Language + snippet without reading geoblock wrappers.

## Risks / Trade-offs

- [Slow Traefik apply > grace] → dispose then recreate. Mitigation: 10s vs ~1 ms measured; tests use a short grace, product stays 10s.
- [Dispose during in-flight Lookup on a dying generation] → possible if a request outlives grace. Mitigation: Traefik has already swapped handlers; 10s covers leftover ServeHTTP. Not the BIN hot-swap race.
- [Copy drift in other repos] → accepted. No new module this change.
- [Yaegi + extra goroutine per Bind] → one watch per `New`. Same generation cancels together. Acceptable vs one shared parent pointer (not exposed on `valueCtx`).

## Migration Plan

- No public Traefik YAML change.
- Roll forward: new binary; old tickers from a previous process die with the process.
- Rollback: previous binary leaks hash-change tickers again (today’s behavior).

## Open Questions

None that change specs. Grace 10s is the explore assumed value, taken here.
