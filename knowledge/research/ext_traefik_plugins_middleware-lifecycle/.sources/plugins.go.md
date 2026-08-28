---
url: https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/middleware/plugins.go
title: pkg/server/middleware/plugins.go
fetched: 2026-08-28
authority: source
ref: github.com/traefik/traefik@83c3499fc31c96e9f80ea0bba4d975f608c7061d:pkg/server/middleware/plugins.go
---

`newTraceablePlugin(ctx, name, plug, next)` immediately calls `plug(ctx, next)` (the `Constructor` / `NewHandler`) and wraps the returned `http.Handler`. No Close on the wrapper.
