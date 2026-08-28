---
url: https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/middleware/middlewares.go
title: pkg/server/middleware/middlewares.go
fetched: 2026-08-28
authority: source
ref: github.com/traefik/traefik@83c3499fc31c96e9f80ea0bba4d975f608c7061d:pkg/server/middleware/middlewares.go
---

`BuildMiddlewareChain(ctx, middlewares)` appends one alice constructor per name. When that constructor runs it calls `buildConstructor` then `constructor(next)`.

For `config.Plugin`, `buildConstructor` calls `pluginBuilder.Build(pluginType, rawPluginConfig, middlewareName)` and returns a constructor that calls `newTraceablePlugin(ctx, middlewareName, plug, next)`.
