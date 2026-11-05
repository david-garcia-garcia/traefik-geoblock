---
url: https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/middlewares/forwardedheaders/forwarded_header.go
title: pkg/middlewares/forwardedheaders/forwarded_header.go
fetched: 2026-11-03
authority: source
ref: github.com/traefik/traefik@faa1eb590646aed94e561e24a59be0c47353ae95:pkg/middlewares/forwardedheaders/forwarded_header.go
---

Comment on `XForwarded`: unless insecure is set, it first removes all existing values for those headers if the remote address is not one of the trusted ones.

Managed set (`XHeadersSet`): `X-Forwarded-Proto`, `X-Forwarded-For`, `X-Forwarded-Host`, `X-Forwarded-Port`, `X-Forwarded-Server`, `X-Forwarded-Uri`, `X-Forwarded-Method`, `X-Forwarded-Prefix`, `X-Forwarded-Tls-Client-Cert`, `X-Forwarded-Tls-Client-Cert-Info`, `X-Real-Ip`. Underscore variants of those names match via `isManagedXHeader`. RFC 7239 `Forwarded` is not in the set.

`NewXForwarded`: builds `ip.Checker` only when `trustedIPs` is non-empty.

`ServeHTTP`: if `!insecure && !isTrustedIP(r.RemoteAddr)` then `DeleteXForwardedHeaders(r.Header)`. Always `rewrite(r)`. Then connection-header cleanup. If `notAppendXForwardedFor`, stashes a context flag for the later reverse proxy. Then `next.ServeHTTP(w, r)` with the same `*http.Request` (RemoteAddr unchanged).

`isTrustedIP`: false when `ipChecker == nil` (empty trustedIPs).

`rewrite`: if `SplitHostPort(RemoteAddr)` succeeds and `X-Real-Ip` is empty, set it to that IP (IPv6 zone stripped). Fill empty Proto/Port/Host from the connection. If any `X-Forwarded-For` values exist, join them with `", "` — no RemoteAddr append. Always set `X-Forwarded-Server` to Traefik hostname.

`DeleteXForwardedHeaders`: deletes every header key that `isManagedXHeader` reports.
