---
url: https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/ip/strategy.go
title: pkg/ip/strategy.go
fetched: 2026-11-03
authority: source
ref: github.com/traefik/traefik@faa1eb590646aed94e561e24a59be0c47353ae95:pkg/ip/strategy.go
---

`Strategy.GetIP(req) string`. None of the strategies write onto `req`.

`RemoteAddrStrategy`: `SplitHostPort(req.RemoteAddr)` (or the raw value on error). Optional IPv6 subnet fold.

`DepthStrategy`: `Header.Get("X-Forwarded-For")`, split on comma, take `xffs[len-depth]` (right-origin). If fewer hops than depth, empty string.

`PoolStrategy` (excludedIPs): split XFF on comma, walk `slices.Backward` (right to left, most recent first), return the first trimmed IP not in `Checker`. All in pool or none parseable → empty string.
