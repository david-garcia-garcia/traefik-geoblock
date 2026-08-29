## Why

`reclaim.Open` takes a house `dispose func(any)` that the table runs after last holder plus grace. That is not a normal Go lifetime. Callers already have contexts and channels; they should watch a context that is canceled at that same moment.

## What Changes

- **BREAKING**: `Open` drops `dispose`. Create is `func(life context.Context) (any, error)`. `life` is canceled when dispose would have run (grace elapsed while orphaned, `Reset`, or a lost create race).
- BIN, MMDB, and Plugin opens stop passing a dispose func. Wrappers start a watch on `life` that calls `close`. Plugin create ignores `life`.
- End-of-life log stays `reclaim_dispose`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_reclaim_context-lease`: `Open` creates once and binds a holder; incarnation end is `life` canceled, not a stored dispose callback.
- `core_geoblock_database_wrapper-reclaim`: wrapper open watches the incarnation lifetime instead of passing dispose.

## Impact

- `pkg/reclaim` `Open` / `Reset` / table tests
- `pkg/dbwrappers` `OpenBIN` / `OpenMMDB` and reclaim tests
- Root `plugin.go` `bindPlugin`
- Usage `std_go_reclaim.md`, wrapper and plugin-instance packets
