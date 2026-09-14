## Context

See proposal.md. DestBranch `pkg/reclaim` discovers `Close()` on stored `any` and owns process `Default`. Traefik-middleware-utilities v1.0.1 (`28da9ab0`) uses `Hooks` (Sleep, Wake, Close), caller-owned `New(Config)`, and no method discovery (Yaegi synthesizes create returns with no methods). BIN and MMDB already run a 24h `dbsource.Updater` ticker that today only stops in `Close`.

## Goals / Non-Goals

**Goals:**
- Copy v1.0.1 `reclaim/table.go` into `pkg/reclaim`. Drop `default.go`.
- Callers pass `Hooks` closing over the pointer assigned inside `create`.
- BIN/MMDB Sleep stops the updater; Wake calls `startUpdate`; Close stops the updater and closes the file/reader.
- Plugin Close still cancels `life`. Plugin Sleep/Wake stay nil.
- Two caller-owned tables (plugin root, `pkg/dbwrappers`). Tests Reset each owner they populated.

**Non-Goals:**
- `go.mod` require of `traefik-middleware-utilities`.
- Importing utilities `yaegi_test.go`.
- `EnforceCloseBeforeOpen` true.
- New plugin YAML keys.
- Inventing Sleep/Wake work Plugin does not already own.

## Decisions

1. **In-tree copy** — Yaegi loads this module’s `pkg/` as GOPATH subpackages; a third-party import is unnecessary for a stdlib-only table already at `pkg/reclaim`.

2. **Sleep/Wake on wrappers** — The table’s asleep window is cheap only if idle work stops. `Updater.Stop` / `Start` already exist. Alternative: Close-only Hooks — rejected; the ticker would keep firing during grace.

3. **Plugin Sleep/Wake nil** — Plugin owns no ticker. Canceling `life` on Sleep would drop wrapper holders while the plugin incarnation is still stored. Close remains the drop of wrapper holders.

4. **Two tables** — Remote has no Default. Prefixes already prevent key collision; separate tables match ownership. Tests that used `dbwrappers.Reset` as process-wide teardown must also Reset the plugin-root table.

5. **`ResetWith` becomes replace** — Remote grace is freeze-at-`New`. Tests that need a short grace call `Reset` then `New(Config{Grace: …})` on that owner.

6. **Wake does not run on first create** — `put` does not call Wake. `newBIN` / `newMMDB` still start the updater once. Wake only restarts after Sleep.

## Risks / Trade-offs

- [Wake `startUpdate` after Sleep] → Mitigation: `dbsource.Start` builds a new Updater; `Stop` is idempotent (closed `stop` chan does not close twice).
- [Plugin tests still call only `dbwrappers.Reset`] → Mitigation: `ResetForTest` on the plugin-root table; `plugin_instance_test.go` resets both.
- [Yaegi still cannot type-switch create `any`] → Mitigation: Hooks funcs close over the concrete pointer; do not restore `closer` discovery.

## Migration Plan

Ship in the next plugin build. No config migration. Operators see the same Traefik `New` reuse; download tickers pause while a wrapper is asleep.

## Open Questions

Ticket questions live on `devstate/explore.md`.
