# Plugin instance

## Language

**Plugin incarnation**:
The shared `Plugin` stored on the plugin-root reclaim table for one Traefik middleware name and normalized config hash.
_Avoid_: treating the returned handler as the stored Plugin; sharing `next`

**Route**:
One Traefik `New`. Holds the shared `*Plugin` and this router’s `next`. `ServeHTTP` on `Route` calls `Plugin.ServeHTTP`.
_Avoid_: copying `Plugin` per route; putting `next` on the stored Plugin; holding catalog lookup on Route

## Overview

Root `New` builds the Plugin once per name+config and reuses it for later routers and same-config reloads. Each `New` returns a `*geoblock.Route`. Format wrappers are held by the Plugin incarnation (`Close` ends that hold), not by each Traefik `New`.

## How to use

- Do not construct maps, regexes, IP helpers, or ban HTML outside `geoblock.NewCore`. That create runs once per incarnation and opens catalog sources only when `mode` is `enrich` or `enrichandblock`.
- Do not bind wrappers to a Traefik `New` context. Plugin `Close` (the table Close hook) cancels `life` when the incarnation ends. Plugin Sleep/Wake are nil.
- Key prefix is `plugin:` on the plugin-root table. Wrappers use a second table (`bin:` / `mmdb:`).
- After grace the table drops the slot and `Close`s the Plugin.
- Tests: `ResetForTest` / `ResetForTestWith` on the plugin-root table, and `dbwrappers.Reset` / `ResetWith` when wrappers were opened. Assert `SameCore` (same `*Plugin`) and per-`New` `Next`. Filter reclaim logs with the `plugin:` prefix.

## Pattern snippet

```go
if err := geoblock.Prepare(cfg, name); err != nil {
	return nil, err
}
stored, err := pluginTable.Open(ctx, pluginKey(name, cfg), logger, func() (any, error) {
	created, err := geoblock.NewCore(name, cfg)
	if err != nil {
		return nil, err
	}
	pluginInstance = created
	return created, nil
}, reclaim.Hooks{Close: func() { pluginInstance.Close() }})
pluginInstance, ok := stored.(*geoblock.Plugin)
return pluginInstance.ForRoute(next)
```

## Key files

- `plugin.go` — `New`, `bindPlugin`, `pluginKey`
- `plugin_instance_test.go` — share, miss, reclaim, grace
- `pkg/geoblock/config.go` — Config, Prepare, catalog bind
- `pkg/geoblock/plugin.go` — NewCore, Plugin, ServeHTTP
- `pkg/geoblock/route.go` — Route, ForRoute
- `vendor/.../traefik-middleware-utilities/reclaim` — plugin-root table vs wrappers table

## Gotchas

- Hash is JSON+FNV of `Config` after `Prepare` (defaults, reserved catalog rows, temp auto-update dir, ban-HTML path search).
- Two middleware names never share, even with the same config.
- Do not write `Table[*Plugin]` (Yaegi).
- `pkg/geoblock` tests use `newTestPlugin` / `newRoute`, not a production `New`. Instance tests must call root `New`.
