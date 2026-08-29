## Context

See proposal.md — Why. Root `New` mutates `Config` via `geoblock.Prepare`, opens a DatabaseProvider, then reuses one Plugin on `reclaim.Default`. Wrappers live under `bin:` / `mmdb:`. `pkg/geoblock` constructs and serves; it does not call `reclaim.Open`. `ServeHTTP` is a value receiver and always uses `p.next`. Wrapper lifecycle tests call `geoblock.New`. Instance tests call root `New`.

## Goals / Non-Goals

**Goals:**

- One Plugin incarnation per middleware name + normalized config hash.
- Keep returning `*Plugin` so existing tests and Yaegi stay the same shape.
- Re-bind wrappers on every `New` so a reused Plugin does not leave wrappers holding only a cancelled generation.

**Non-Goals:**

- Changing `Table.Open` to a lifetime context (follow-up).
- A second reclaim table.
- Sharing `next` across routers.
- New public config keys.

## Decisions

1. **Same Default table, prefix `plugin:`.**
   Alternative: a second table — rejected; `std_go_reclaim` already says prefix keys when more than one type shares Default.

2. **Key is `plugin:` + name + `:` + hex hash of Config after New’s existing mutate steps.**
   Whole `Config` via `encoding/json` + FNV-64a (same algorithm as wrapper `configHash`). `encoding/json` sorts map keys, so the hash is stable. Name is the Traefik middleware name, not hashed into the body so logs stay readable.

3. **Store `*geoblock.Plugin` with `next` nil. Each root `New` calls `ForRoute(next, db)`.**
   Alternative: a new `routeHandler` type — rejected; tests still type-assert `*geoblock.Plugin`.
   Alternative: mutate `next` on the stored Plugin — rejected; two routers would share one next.
   Instance reclaim stays in the Yaegi root. `pkg/geoblock` is construct + ServeHTTP only.

4. **Always `geoblock.OpenDatabase` before `reclaim.Open`.**
   First put order stays wrapper-then-plugin when tests call root `New`. The copy’s `db` is this `New`’s provider so lookups use wrappers bound to this context.

5. **Plugin dispose is empty.**
   The incarnation has no keep-current loop. After grace the table drops it; maps and IP helpers become collectable when no copy remains. Wrappers dispose on their own keys.

6. **Yaegi: `reclaim.Open` + type-assert `*Plugin`.**
   Do not write `Table[*Plugin]`.

## Risks / Trade-offs

- [Lifecycle tests count all `reclaim_put`] → plugin puts add lines; reuse/reclaim assertions still hold if create runs once. Filter by `plugin:` prefix in new tests.
- [Hash includes temp `databaseAutoUpdateDir`] → that path is process-wide (`os.TempDir()` + fixed name), so two `New` with empty dir still match.
- [Shallow copy shares maps] → allow/block maps are immutable after `New`; do not mutate them on the request path (already true).

## Migration Plan

No operator config change. Reload behavior is the same (cancel, then `New`). Rollback is revert the binary.

## Open Questions

None. Deferred Table lifetime-context is on `devstate/issues.md`.
