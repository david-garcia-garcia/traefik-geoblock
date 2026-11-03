---
url: https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/config/static/entrypoints.go
title: pkg/config/static/entrypoints.go (ForwardedHeaders)
fetched: 2026-11-03
authority: source
ref: github.com/traefik/traefik@faa1eb590646aed94e561e24a59be0c47353ae95:pkg/config/static/entrypoints.go
---

`ForwardedHeaders` struct: `Insecure` “Trust all forwarded headers.”; `TrustedIPs` “Trust only forwarded headers from selected IPs.”; `Connection`; `NotAppendXForwardedFor` “Disable appending RemoteAddr to X-Forwarded-For header. Defaults to false (appending is enabled).”

Separate `ProxyProtocol` struct: `Insecure` “Trust all.”; `TrustedIPs` “Trust only selected IPs.”
