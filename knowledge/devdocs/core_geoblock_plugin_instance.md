# Plugin instance

## Language

**Plugin incarnation**:
The shared `Plugin` stored on the process reclaim table for one Traefik middleware name and normalized config hash.
_Avoid_: treating each returned `*Plugin` as a distinct core (it is a shallow copy); sharing `next`

## Overview

`New` builds the Plugin once per name+config and reuses it for later routers and same-config reloads. Each `New` still returns a `*Plugin` whose `next` is that router’s chain and whose provider was opened with that `New` context.

## How to use

- Do not construct maps, regexes, IP helpers, or ban HTML outside `newPluginCore`. That create runs once per incarnation.
- Always open the DatabaseProvider with this `New` context before `reclaim.Open`, so wrappers bind even on reuse.
- Key prefix is `plugin:`. Same process table as `bin:` / `mmdb:` (`std_go_reclaim.md`).
- Dispose is empty. After grace the table drops the incarnation.
- Tests: `dbwrappers.Reset` / `ResetWith`. Assert shared pointers (`allowedIPBlocks`, country maps) and per-`New` `next`. Filter reclaim logs with the `plugin:` prefix.

## Pattern snippet

```go
v, err := reclaim.Open(ctx, pluginKey(name, cfg), func() (any, error) {
	return newPluginCore(name, cfg, db, logger, enrich)
}, func(any) {})
out := *(v.(*Plugin))
out.next = next
out.db = db
```

## Key files

- `plugin.go` — `New`, `bindPlugin`, `pluginKey`
- `plugin_lifecycle_test.go` — share, miss, reclaim, grace
- `pkg/reclaim` — process table

## Gotchas

- Hash is JSON+FNV of `Config` after defaults, pointer fallbacks, empty-source binds, temp auto-update dir, and ban-HTML path search.
- Two middleware names never share, even with the same config.
- Do not write `Table[*Plugin]` (Yaegi).
