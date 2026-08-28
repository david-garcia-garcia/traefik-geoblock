---
url: https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/plugins/middlewareyaegi.go
title: pkg/plugins/middlewareyaegi.go
fetched: 2026-08-28
authority: source
ref: github.com/traefik/traefik@83c3499fc31c96e9f80ea0bba4d975f608c7061d:pkg/plugins/middlewareyaegi.go
---

`newYaegiMiddlewareBuilder` Evals `basePkg.New` and `basePkg.CreateConfig` once on the interpreter.

`newMiddleware` calls `CreateConfig`, mapstructure-decodes the dynamic plugin map into that value, and returns a `YaegiMiddleware` holding the decoded config. It does not call plugin `New`.

`newHandler` calls plugin `New` as `(ctx, next, cfg, middlewareName)` and requires an `http.Handler` (and optional error). No `Close` lookup.

`YaegiMiddleware.NewHandler` forwards to `newHandler`.
