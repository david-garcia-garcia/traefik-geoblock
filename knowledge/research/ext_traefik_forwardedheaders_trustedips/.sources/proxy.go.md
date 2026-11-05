---
url: https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/proxy/httputil/proxy.go
title: pkg/proxy/httputil/proxy.go
fetched: 2026-11-03
authority: source
ref: github.com/traefik/traefik@faa1eb590646aed94e561e24a59be0c47353ae95:pkg/proxy/httputil/proxy.go
---

Reverse-proxy `Rewrite`: `copyForwardedHeader` copies `X-Forwarded-For`, `Forwarded`, `X-Forwarded-Host`, `X-Forwarded-Proto` from in to out.

Unless context `NotAppendXFF` is set (from `forwardedHeaders.notAppendXForwardedFor`), `SplitHostPort(RemoteAddr)` is appended to outgoing `X-Forwarded-For` (joined with existing values by `", "`). If no prior XFF, the outgoing header becomes that client IP alone.

This runs when Traefik talks to the backend, after middleware (including plugins) has already seen the request.
