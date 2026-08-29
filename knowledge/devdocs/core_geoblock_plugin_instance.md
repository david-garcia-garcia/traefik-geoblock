# Plugin instance

## Language

**Plugin incarnation**:
The shared `Plugin` stored on the process reclaim table for one Traefik middleware name and normalized config hash.
_Avoid_: treating each returned `*Plugin` as a distinct core (it is a shallow copy); sharing `next`

## Overview

Root `New` builds the Plugin once per name+config and reuses it for later routers and same-config reloads. Each `New` still returns a `*geoblock.Plugin` whose `next` is that router’s chain. Wrapper bind is `ForRoute`, not the instance table.

## How to use

- Do not construct maps, regexes, IP helpers, or ban HTML outside `geoblock.NewCore`. That create runs once per incarnation.
- Do not open the DatabaseProvider in the Yaegi root. `ForRoute` opens it with this `New` context so wrappers bind even on reuse.
- Key prefix is `plugin:`. Same process table as `bin:` / `mmdb:` (`std_go_reclaim.md`).
- Dispose is empty. After grace the table drops the incarnation.
- Tests: `dbwrappers.Reset` / `ResetWith` from the root package. Assert `SameCore` and per-`New` `Next`. Filter reclaim logs with the `plugin:` prefix.

## Pattern snippet

```go
if err := geoblock.Prepare(cfg, name); err != nil {
	return nil, err
}
v, err := reclaim.Open(ctx, pluginKey(name, cfg), func() (any, error) {
	return geoblock.NewCore(name, cfg)
}, func(any) {})
return v.(*geoblock.Plugin).ForRoute(ctx, next, cfg)
```

## Key files

- `plugin.go` — `New`, `bindPlugin`, `pluginKey`
- `plugin_instance_test.go` — share, miss, reclaim, grace
- `pkg/geoblock` — Prepare, NewCore, ForRoute
- `pkg/reclaim` — process table

## Gotchas

- Hash is JSON+FNV of `Config` after `Prepare` (defaults, pointer fallbacks, empty-source binds, temp auto-update dir, ban-HTML path search).
- Two middleware names never share, even with the same config.
- Do not write `Table[*Plugin]` (Yaegi).
- `pkg/geoblock.New` constructs one Plugin and does not reuse. Instance tests must call root `New`.
