---
url: https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/plugins/middlewareyaegi.go
title: pkg/plugins/middlewareyaegi.go
fetched: 2026-11-03
authority: source
ref: github.com/traefik/traefik@faa1eb590646aed94e561e24a59be0c47353ae95:pkg/plugins/middlewareyaegi.go
---

Yaegi builder evals plugin `New` and `CreateConfig`. `newHandler` calls `New(ctx, next, cfg, middlewareName)` and requires the first return to be `http.Handler`.

No client-IP argument, request wrapper type, or extra header is introduced here. The plugin handler is used as a normal `http.Handler` in the middleware chain, so `ServeHTTP` sees the `*http.Request` the entrypoint already mutated.
