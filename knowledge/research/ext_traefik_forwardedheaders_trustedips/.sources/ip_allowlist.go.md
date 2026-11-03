---
url: https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/middlewares/ipallowlist/ip_allowlist.go
title: pkg/middlewares/ipallowlist/ip_allowlist.go
fetched: 2026-11-03
authority: source
ref: github.com/traefik/traefik@faa1eb590646aed94e561e24a59be0c47353ae95:pkg/middlewares/ipallowlist/ip_allowlist.go
---

`ServeHTTP`: `clientIP := al.strategy.GetIP(req)`; authorize against `sourceRange`; on failure write reject status and return; on success `al.next.ServeHTTP(rw, req)` with the same request. `clientIP` is not stored on headers, context, or `RemoteAddr`.
