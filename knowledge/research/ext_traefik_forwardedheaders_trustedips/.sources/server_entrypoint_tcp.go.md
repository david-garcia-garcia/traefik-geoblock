---
url: https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/server/server_entrypoint_tcp.go
title: pkg/server/server_entrypoint_tcp.go
fetched: 2026-11-03
authority: source
ref: github.com/traefik/traefik@faa1eb590646aed94e561e24a59be0c47353ae95:pkg/server/server_entrypoint_tcp.go
---

`newHTTPServer`: `httpSwitcher` is the live router mux. `alice` wraps it with `requestdecorator`. `forwardedheaders.NewXForwarded(Insecure, TrustedIPs, Connection, NotAppendXForwardedFor, next)` wraps that. Later wrappers (path, content-type, …) sit outside. Every HTTP request hits `XForwarded.ServeHTTP` before plugin middleware on the switcher.

`buildProxyProtocolListener` (separate from forwardedHeaders): if `ProxyProtocol.Insecure`, wrap the TCP listener with go-proxyproto and trust every connection. Else require `TrustedIPs`; untrusted upstream → `proxyproto.IGNORE` (do not use PROXY headers). Trusted → `USE`. This is what can replace the connection remote address the HTTP stack later stores as `req.RemoteAddr`.
