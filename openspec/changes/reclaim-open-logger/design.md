## Context

See proposal.md — Why. `Table.Open` and package `Open` take `(ctx, key, create)`. `Default()` embeds `processLogger` at info. `table.go` must stay stdlib-only. Yaegi cannot call `func(context.Context)` as `create`; an extra `*slog.Logger` argument is assumed loadable. `plugin.go` `bindPlugin` calls `Open` before `NewCore`, so it has no plugin logger unless it builds one first.

## Goals / Non-Goals

**Goals:**
- Every `Open` carries the caller’s logger; put/bind/reclaim use that call; orphan/dispose use the last bind’s logger.
- Put and dispose at debug so plugin `logLevel` gates them.
- Production call sites (`plugin.go`, `OpenBIN`, `OpenMMDB`) pass the middleware logger.

**Non-Goals:**
- Removing `Default` or the table-wide logger used by `NewTable` / `Reset`.
- New plugin config keys.
- Importing `pkg/logging` from `table.go`.

## Decisions

1. **Signature `Open(ctx, key, logger, create)`** — logger is a concrete `*slog.Logger`, not a create-func argument. Alternative: process `UseLogger` (first middleware wins) — rejected; ticket chose per-Open.

2. **Nil logger → table logger → `slog.Default()`** — same fallback as `NewTable`. Alternative: panic — rejected; tests and a missed call site should still run.

3. **Slot stores last Open logger** — orphan/dispose fire after the caller is gone. Alternative: creator-only — rejected; last bind matches “this middleware’s level” when two holders share a key.

4. **`bindPlugin` builds the plugin logger before `Open`** — export or reuse `pluginLogger` so a reclaim (no `create`) still logs bind/reclaim at this `New`’s `logLevel`. Alternative: logger only inside `create` — rejected; reclaim would keep the previous incarnation’s logger for bind.

5. **Keep `processLogger` on `Default()`** — fallback for nil Open logger and `Reset` of slots that never stored one. Not used when callers pass a logger.

## Risks / Trade-offs

- [Yaegi rejects extra Open arg] → Mitigation: assumed loadable; if CI Yaegi tests fail, keep signature and fix the load path, do not drop the logger.
- [Shared BIN/MMDB key, two logLevels] → Mitigation: last Open wins for orphan/dispose; put still uses the creating Open.
- [Operators lose info-level put/dispose] → Mitigation: intended; document in usage packet.

## Migration Plan

Ship in the next plugin build. Callers in this module are updated in the same change. No config migration. Dashboards that alert on info `reclaim_put` must use debug.

## Open Questions

None. Ticket questions are on `devstate/explore.md`.
