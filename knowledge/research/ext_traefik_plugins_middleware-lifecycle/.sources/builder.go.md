---
url: https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/plugins/builder.go
title: pkg/plugins/builder.go
fetched: 2026-08-28
authority: source
ref: github.com/traefik/traefik@83c3499fc31c96e9f80ea0bba4d975f608c7061d:pkg/plugins/builder.go
---

`Constructor` is `func(context.Context, http.Handler) (http.Handler, error)`.

`pluginMiddleware` is only `NewHandler(ctx, next)`. There is no `Close` / `Stop` on this interface.

`NewBuilder` walks static catalog plugins and `localPlugins` once and stores one `middlewareBuilder` per plugin name (Yaegi interpreter or WASM builder).

`Build(pName, config, middlewareName)` calls `newMiddleware` then returns `m.NewHandler`.
