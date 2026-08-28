---
url: https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/routerfactory.go
title: pkg/server/routerfactory.go
fetched: 2026-08-28
authority: source
ref: github.com/traefik/traefik@83c3499fc31c96e9f80ea0bba4d975f608c7061d:pkg/server/routerfactory.go
---

`CreateRouters`:

1. If `cancelPrevState != nil`, call it.
2. `ctx, f.cancelPrevState = context.WithCancel(context.Background())`.
3. New service manager, `middleware.NewBuilder`, `router.NewManager`.
4. `BuildHandlers(ctx, …)` for HTTP (non-TLS then TLS).

The same `ctx` is the one passed down into plugin `New`. The next `CreateRouters` cancels it before building the replacement handlers. No equality check on plugin config inside this function.
