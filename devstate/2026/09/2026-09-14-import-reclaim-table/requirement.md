# Requirement
IssueKey: 2026-09-14-import-reclaim-table

## Problem
This repo keeps a custom reclaim table that discovers `Close()` on the stored value. The ticket asks to import the reclaim table from traefik-middleware-utilities v1.0.1 and reshape this copy and its callers to that surface, including the remote `Hooks` shape (Sleep, Wake, Close). Done when the PR is passing CI and the delivery card is updated.

## Current (code)
- `pkg/reclaim/table.go`: `Table.Open(ctx, key, logger, create)` has no `Hooks` argument. A successful create stores `any` and starts a cancelable lifetime; dispose discovers `Close()` on that value (`closer` / `stopValue`).
- `pkg/reclaim/default.go`: process singleton `Default`, package `Open`, `Reset`, `ResetWith`, and `NewTable(grace)`.
- `plugin.go` `bindPlugin`: `reclaim.Open(ctx, pluginKey(...), logger, create)` with no hooks. `*geoblock.Plugin` has `Close()` at `pkg/geoblock/plugin.go`.
- `pkg/dbwrappers/bin.go` `OpenBIN`: `reclaim.Open` then assert `*BIN`. `BIN.Close()` stops the updater and file handle; the table is expected to call it.
- `pkg/dbwrappers/mmdb.go` `OpenMMDB`: same Close-only pattern for `*MMDB`.
- `pkg/dbwrappers/reset.go`: tests call package `reclaim.Reset` / `ResetWith`.
- `knowledge/devdocs/std_go_reclaim.md`: documents Default / `Open` / Close discovery, not Sleep/Wake hooks.
- `knowledge/research/ext_traefik-middleware-utilities_reclaim-table/notes.md`: pinned v1.0.1 (`28da9ab0`) public surface is `New(Config)`, `Open(..., hooks Hooks)`, no Default, no Close() discovery.

## Desired
Replace this project's reclaim table with the v1.0.1 shape (or an in-tree copy of it) and adjust callers to that shape. The table we will use DOES have Sleep, Wake, and Close hooks: import and wire the v1.0.1 `Hooks` API (`Sleep`, `Wake`, `Close`). Do not keep Close-only method discovery as the target. Reshape local callers (`plugin.go`, `OpenBIN`, `OpenMMDB`, test resets) to pass `Hooks`. Accept other remote differences (caller-owned `New(Config)`, no process `Default`, canceled-bind error, create-once, hook panic recovery) as needed so the import matches the more flexible remote component. Finished when the PR is passing CI and the delivery card is updated.

## Affected
- `pkg/reclaim` (`table.go`, `default.go`, `table_test.go`)
- `plugin.go` (`bindPlugin`)
- `pkg/dbwrappers` (`bin.go`, `mmdb.go`, `reset.go` and their tests)
- `pkg/geoblock` plugin Close / instance tests that assert reclaim msgs
- `openspec/specs/std_go_reclaim_context-lease/spec.md` and `knowledge/devdocs/std_go_reclaim.md` (later phases)

## Out of scope
- Geo lookup policy, catalog sources, Traefik plugin YAML, and request-mode behavior
- Adding Sleep/Wake side effects the stored types do not already own, beyond wiring `Hooks` so the table can call them
- Rewriting unrelated vendor Close methods
- Opening a second PR or treating the run done before CI and the delivery card

## Unknowns
- Copy v1.0.1 into `pkg/reclaim` versus `go.mod` require of `github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim` (Yaegi plugin packaging may force an in-tree copy)
- Per-caller hook bodies: which of Plugin / BIN / MMDB need non-nil Sleep and Wake versus a Close-only `Hooks{Close: ...}` once the API is `Hooks`

## Tensions
- Local Close() discovery + lifetime context vs remote `Hooks` and no create lifetime (`notes.md` vs `pkg/reclaim/table.go`)
- Local process `Default` / `NewTable` / `ResetWith` vs remote caller-owned `New(Config)` only
- Human text requires Sleep/Wake/Close hooks; current callers only implement Close
