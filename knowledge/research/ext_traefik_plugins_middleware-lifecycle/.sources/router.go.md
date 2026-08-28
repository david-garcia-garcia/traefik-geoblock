---
url: https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/router/router.go
title: pkg/server/router/router.go
fetched: 2026-08-28
authority: source
ref: github.com/traefik/traefik@83c3499fc31c96e9f80ea0bba4d975f608c7061d:pkg/server/router/router.go
---

`buildHTTPHandler` builds the alice chain for **that router**, then `mHandler := middlewaresBuilder.BuildMiddlewareChain(ctx, router.Middlewares)` and `chain.Extend(*mHandler).Then(nextHandler)`.

`Then` materializes constructors during handler build (config apply), not per request.

`buildRouterHandler` caches the handler on the current `Manager` by router name so the same router is not built twice inside one `CreateRouters`. A new `Manager` is created on each `CreateRouters`, so the cache does not survive reload.
