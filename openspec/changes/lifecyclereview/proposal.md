## Why

Traefik cancels the `New` context for a whole router generation, then calls `New` again (~1 ms later), even when plugin YAML did not change. Hash-shared BIN/MMDB wrappers start a download ticker that never stops: vendor `Close` is a no-op, and Traefik never closes the handler. Same-hash reload is safe only by accident (singleton). A hash change leaks the old ticker. Other projects that start background work in `New` need the same cancel-then-reclaim rule, not a geoblock-specific factory.

## What Changes

- Add a **stdlib-only** reclaim package (`pkg/reclaim`): `Put` (one dispose per incarnation) and `Bind` (one context per holder), grace timer, reclaim cancels dispose. No plugin, wrapper, or vendor imports. Another repo can copy the package.
- `OpenBIN` / `OpenMMDB` take the `New` context and bind the wrapper hash to that package. Unreclaimed hash (config gone or last router dropped) disposes after grace: stop updater, close file, drop the map entry.
- Same-hash `New` after cancel reclaims and does **not** dispose.
- Plugin `New` passes `ctx` into the provider open path.
- BIN hot-swap 10s delayed close stays. Not this change.
- OOTB LITE defaults and README are not this change.

## Capabilities

### New Capabilities

- `std_go_reclaim_context-lease`: stdlib keyed reclaim — bind contexts, grace, dispose callback. No product types.
- `core_geoblock_database_wrapper-reclaim`: format wrappers open with `New` ctx and dispose through reclaim; hash-change leak ends.

### Modified Capabilities

None.

## Impact

- New `pkg/reclaim` (stdlib only)
- `pkg/dbwrappers` `OpenBIN` / `OpenMMDB` take `ctx`; bind + dispose
- `plugin.go` `openDatabaseProvider` and vendor `New` pass `ctx`
- Tests: reclaim unit tests (no Traefik); wrapper reclaim / hash-evict
- Usage: `knowledge/devdocs/std_go_reclaim.md`; wrapper packet `Open` grows a `ctx`
- Specs: two new leaves (above)
