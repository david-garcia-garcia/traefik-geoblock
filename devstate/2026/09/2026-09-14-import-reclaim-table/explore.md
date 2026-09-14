# Explore
IssueKey: 2026-09-14-import-reclaim-table
Verdict: in progress

## Concepts

This product already has a keyed reclaim table in `pkg/reclaim`. Traefik `New` and catalog wrappers store one `any` per key, survive a reload if the same key is opened again before grace, and today discover `Close()` on the stored value. The ticket is to import the pinned v1.0.1 surface from traefik-middleware-utilities (`28da9ab0`) and reshape this copy and its callers to that surface. The human already closed the table-shape unknown: the table we import **does** have `Hooks` with Sleep, Wake, and Close. Do not keep Close-only method discovery as the target.

```
DestBranch today                         v1.0.1 (target)
----------------                         ---------------
reclaim.Open(ctx,key,logger,create)      table.Open(ctx,key,logger,create,hooks)
Default / NewTable / ResetWith           New(Config) only; caller holds *Table
create -> LIVE -> grace -> cancel(life)  create -> (sleep <-> wake)* -> sleep -> close
stopValue: v.(closer).Close()             Hooks.Close (nil Sleep/Wake skip)
lost-create race can create twice        key busy before create; one create
(value, nil) even if ctx already Done    (nil, ctx.Err()) if bind ctx is canceled
```

**Table.** Non-generic keyed store of `any`. Callers type-assert. Yaegi cannot instantiate `otherpkg.Table[*T]`. House packet: `knowledge/devdocs/std_go_reclaim.md`. Remote: `knowledge/research/ext_traefik-middleware-utilities_reclaim-table/notes.md`.

**Hooks.** Optional `Sleep`, `Wake`, `Close` `func()` plus `EnforceCloseBeforeOpen`. Stored at first put; a later `Open` for the same incarnation ignores its `hooks` argument. A nil func skips that event. The table must not type-switch the stored `any` for those events. Yaegi v0.16.1 synthesizes a `create` return with no methods, so today’s `closer` assert does not match under the interpreter (`notes.md` Yaegi constraint 3; `TestYaegi_CreateAnyTypeSwitchDoesNotMatch`). Pass funcs that close over a pointer assigned inside `create`.

**Open.** `create` takes no arguments. Logger required. First Open registers the key busy before `create`. Concurrent first Opens share one create. `(value, nil)` means a live bind; `ctx.Err()` at bind returns `(nil, ctx.Err())` and drops that holder.

**Grace.** How long a **sleeping** value is kept before Close. Negative `Config.Grace` → `DefaultGrace` (10s). Zero: Sleep then Close back to back, no reclaim window. Waiter is stdlib `time.AfterFunc` (Yaegi `select` can miss the timer).

**Default.** Process singleton (`reclaim.Default`, package `Open`, `Reset`, `ResetWith`, `NewTable`). Remote: absent. Callers construct with `New(Config)` and hold the `*Table`.

**Plugin incarnation.** Root `bindPlugin` stores `*geoblock.Plugin` on key `plugin:<name>:<hash>`. `Plugin.Close` cancels the Plugin’s own `life` context so BIN/MMDB holders drop. `NewCore` already creates that `life` itself; reclaim does not hand `create` a lifetime today, and v1.0.1 does not either.

**Wrappers.** `OpenBIN` / `OpenMMDB` store `*BIN` / `*MMDB` on `bin:` / `mmdb:` keys. Each type has `Close()` (stop updater + file/reader). Neither type has Sleep or Wake. Updater `Stop` exists only as part of Close.

**Test Reset.** `dbwrappers.Reset` / `ResetWith` tear down the process table. Plugin instance/lifecycle/config tests call those helpers even for `plugin:` keys, because today one Default holds both.

Production `reclaim.Open` call sites (searched `*.go` excluding `vendor/`): `plugin.go` `bindPlugin`, `pkg/dbwrappers/bin.go` `OpenBIN`, `pkg/dbwrappers/mmdb.go` `OpenMMDB` (3). Package `Open` is the only production entry; `Table.Open` is also used from `pkg/reclaim/table_test.go`.

## Decisions

- Import the v1.0.1 `Hooks` API. Sleep, Wake, and Close exist on the table. Close-only method discovery (`closer` / `stopValue`) is not the target. Human Desired closed this.
- Copy v1.0.1 into `pkg/reclaim` (replace `table.go`, drop `default.go` Default / package `Open` / `ResetWith` / `NewTable`). Do not `go.mod` require `github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim`. Yaegi loads this module’s `pkg/` as GOPATH subpackages (`knowledge/research/ext_traefik_plugins_yaegi-subpackages/notes.md`). Third-party modules are “vendored, Go modules not supported” for Traefik plugins; an extra import path is unnecessary for a stdlib-only table this repo already owns at `pkg/reclaim`.
- Reshape callers onto Sleep / Wake / Close. BIN and MMDB already own a 24h `dbsource.Updater` ticker: Sleep stops it, Wake starts it again (`startUpdate`), Close stops it and closes the file/reader. Plugin has no ticker; Sleep and Wake stay nil; Close still cancels `life` so wrapper holders drop. Human: reshape includes moving existing components onto what the table offers. Out of scope does not cover leaving the updater running through grace.
- Drop the process Default. Plugin root and `pkg/dbwrappers` each hold `var table = reclaim.New(reclaim.Config{Grace: reclaim.DefaultGrace})`. Keys stay prefixed (`plugin:` / `bin:` / `mmdb:`), so two tables do not share incarnations. Tests that used `dbwrappers.Reset` / `ResetWith` as a process-wide teardown must Reset (or replace via `New(Config{Grace: short})`) **each** owner they populated. `ResetWith` is not on the remote table; grace is freeze-at-`New`.
- Leave `EnforceCloseBeforeOpen` false (Hooks default). BIN opens a file handle but not an exclusive lock; MMDB is `FromBytes` (no mmap). Overlapping Close vs next create is the remote default.
- Keep `Plugin`’s own `life` / `Close` as the holder context for wrappers. Do not invent a reclaim create-lifetime. Remote `create` takes no args.
- Do not import utilities `yaegi_test.go` (Yaegi interp require). Reshape this repo’s `table_test.go` to `New` + `Hooks`. Later phases update `openspec/specs/std_go_reclaim_context-lease/spec.md` and `knowledge/devdocs/std_go_reclaim.md`.

This work does not set or reconstruct client address, user, tenant, Host, or trust hop. Those stay with existing owners (`iplookup` / Traefik `New` ctx as holder only).

## Open questions

- Q: Does the reclaim table we import have Sleep, Wake, and Close hooks, or is Close-only discovery the target?
  Rank: structural asked — replaces the Close-discovery table; Desired “The table we will use DOES have Sleep, Wake, and Close hooks”
  Decision: resolved — import and wire the v1.0.1 `Hooks` API (`Sleep`, `Wake`, `Close`). Do not keep Close-only method discovery.
  By: explore

- Q: Copy v1.0.1 into `pkg/reclaim`, or `go.mod` require `github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim`?
  Rank: structural asked — replaces existing `pkg/reclaim`; Desired “Replace this project's reclaim table with the v1.0.1 shape (or an in-tree copy of it)”
  Decision: assumed — copy the pinned v1.0.1 sources into `pkg/reclaim`. Do not add a module require. Yaegi already loads own-module `pkg/`; official plugin docs require vendoring for third-party modules (`ext_traefik_plugins_yaegi-subpackages`); the table is stdlib-only so the copy is the published file.
  By: explore

- Q: Which of Plugin / BIN / MMDB need non-nil Sleep and Wake versus a Close-only `Hooks{Close: ...}` once the API is `Hooks`?
  Rank: bounded asked — 3 production `reclaim.Open` sites (`plugin.go`, `pkg/dbwrappers/bin.go`, `pkg/dbwrappers/mmdb.go`); Desired “adjust to its shape”; human: reshape includes moving existing components onto Sleep/Wake so timers stop while asleep
  Decision: resolved — BIN and MMDB pass Sleep (updater.Stop) and Wake (startUpdate). Plugin Sleep/Wake nil (no ticker); Close cancels `life`. Close still disposes the file/reader (wrappers) or life (plugin).
  By: propose

- Q: After Default goes away, who holds the `*Table`, and how do tests that call `dbwrappers.Reset` / `ResetWith` still tear down plugin and wrapper incarnations?
  Rank: bounded asked — 3 production Opens plus test Reset helpers; Desired “Accept other remote differences (caller-owned New(Config), no process Default…)”; searched `*.go` excluding `vendor/` for `reclaim.Open`, `dbwrappers.Reset`, `ResetWith`
  Decision: assumed — plugin root holds one table; `pkg/dbwrappers` holds one table. Production Opens (3) switch from package `Open` to that owner’s `table.Open`. `dbwrappers.Reset` resets only the wrappers table. `ResetWith(grace)` becomes Reset plus replace that owner’s table with `New(Config{Grace: grace})`. Plugin tests in `plugin_instance_test.go`, `pkg/geoblock/plugin_lifecycle_test.go`, and `pkg/geoblock/plugin_config_test.go` that today call `dbwrappers.Reset` to clear `plugin:` keys must also Reset the plugin-root table. Wrapper tests (`pkg/dbwrappers/{reset.go,reclaim_test.go,bin_test.go,bin_record_test.go,mmdb_test.go,geoip2_test.go,ipinfo_test.go}`) stay on the wrappers table.
  By: explore

- Q: Should any caller set `Hooks.EnforceCloseBeforeOpen`?
  Rank: additive asked — new field on `Hooks` this change introduces; Desired “Accept other remote differences … as needed so the import matches”
  Decision: assumed — false for Plugin, BIN, and MMDB (remote default). BIN is a file handle without an exclusive lock; MMDB is memory (`FromBytes`); Plugin.Close only cancels `life`.
  By: explore
