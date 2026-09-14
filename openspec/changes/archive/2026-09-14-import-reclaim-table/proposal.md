## Why

This plugin’s reclaim table discovers `Close()` on stored `any` and owns a process `Default`. Yaegi synthesizes create returns with no methods, so that discovery does not run under Traefik. The pinned utilities v1.0.1 table uses `Hooks` (Sleep, Wake, Close) and caller-owned `New(Config)` instead.

## What Changes

- **BREAKING**: Replace `pkg/reclaim` with an in-tree copy of traefik-middleware-utilities v1.0.1 (`28da9ab0`). `Open` takes `hooks Hooks`. No `Close()` discovery, no create lifetime, no process `Default` / package `Open` / `ResetWith` / `NewTable`.
- Production Opens close over the pointer assigned inside `create`. BIN and MMDB pass Sleep (`updater.Stop`) and Wake (`startUpdate`) so the 24h keep-current ticker does not run while the value is asleep; Close still stops the updater and closes the file/reader. Plugin Sleep/Wake stay nil (no ticker); Close cancels `life`. `EnforceCloseBeforeOpen` stays false.
- Plugin root holds one `*Table`; `pkg/dbwrappers` holds one `*Table`. Tests Reset (or replace via `New(Config{Grace: …})`) each owner they populated.
- Do not `go.mod` require `github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim`. Do not import utilities `yaegi_test.go`.

## Capabilities

### New Capabilities

- `std_go_reclaim_value-lifecycle`: stored-value events are create → (sleep → wake)* → sleep → close via `Hooks`; the table does not type-switch `any`.

### Modified Capabilities

- `std_go_reclaim_context-lease`: caller-owned `New(Config)` tables; `Open` registers before create, returns `(nil, ctx.Err())` on canceled bind, grace is a sleeping window.
- `core_geoblock_plugin_instance-reclaim`: plugin-root table; Close cancels `life`; tests Reset that table (and wrappers when they populated them).
- `core_geoblock_database_wrapper-reclaim`: wrappers table; Sleep/Wake pause and resume the keep-current ticker; `dbwrappers.Reset` tears down that table only.

## Impact

- `pkg/reclaim` (`table.go`, drop `default.go`, reshape `table_test.go`)
- Root `plugin.go` `bindPlugin` and `plugin_instance_test.go`
- `pkg/dbwrappers` (`bin.go`, `mmdb.go`, `reset.go` and wrapper tests)
- `pkg/geoblock` lifecycle/config tests that call `dbwrappers.Reset` / `ResetWith`
- Usage packets `std_go_reclaim.md`, `core_geoblock_plugin_instance.md`, `core_geoblock_database_wrapper.md` (implement / later phases)
