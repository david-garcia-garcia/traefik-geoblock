## Why

The process reclaim table builds its own info-level stdout logger. `reclaim_put` and `reclaim_dispose` therefore ignore each middleware’s `logLevel`. Operators cannot keep those lines at debug.

## What Changes

- **BREAKING**: `Open` (package and `Table`) takes a `*slog.Logger` after `key`. Callers pass the middleware logger.
- `reclaim_put` and `reclaim_dispose` are debug (all five reclaim messages at debug).
- `plugin.go`, `OpenBIN`, and `OpenMMDB` pass the logger they already have (plugin constructor built before `Open` so a reclaim still uses this middleware’s level).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_reclaim_context-lease`: `Open` accepts a logger; put/dispose at debug; nil logger falls back to the table logger.

## Impact

- `pkg/reclaim` (`table.go`, `default.go`, tests)
- `plugin.go` Yaegi `Open` call
- `pkg/dbwrappers` `OpenBIN` / `OpenMMDB`
- Usage packet `knowledge/devdocs/std_go_reclaim.md`
