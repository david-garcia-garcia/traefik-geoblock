# ForwardedHeaders trustedIPs

Where Traefik (the host) sanitizes `X-Forwarded-*` / `X-Real-IP` at the entrypoint, and what a Yaegi HTTP plugin then sees. Not how this product reconstructs a client hop.

Pinned implementation: [traefik@faa1eb59](https://github.com/traefik/traefik/tree/faa1eb590646aed94e561e24a59be0c47353ae95) (`v3.7.11`). Official field table: [EntryPoints v3.7](https://doc.traefik.io/traefik/v3.7/reference/install-configuration/entrypoints/). Trust-boundary position: [Security Decisions](https://doc.traefik.io/traefik/contributing/security-decisions/).

v3.5 vs v3.7.11: `trustedIPs` and `insecure` have the same official wording on [EntryPoints v3.5](https://doc.traefik.io/traefik/v3.5/reference/install-configuration/entrypoints/). v3.7.11 adds `forwardedHeaders.connection` and `forwardedHeaders.notAppendXForwardedFor` (absent from the v3.5 field table). Follow this pin for what `traefik:v3.7.11` does.

## Trust is decided once, before any middleware

`forwardedHeaders.trustedIPs` and `forwardedHeaders.insecure` decide once, at the entrypoint, **before any middleware runs**, whether a client's forwarding headers are trusted. Traefik does not re-decide per middleware ([Security Decisions](https://doc.traefik.io/traefik/contributing/security-decisions/)).

The HTTP server wraps the router switcher with `forwardedheaders.NewXForwarded(...)` so `XForwarded.ServeHTTP` runs on every request before plugin middleware ([traefik@faa1eb59:pkg/server/server_entrypoint_tcp.go](https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/server/server_entrypoint_tcp.go)).

Trust check: `!insecure && !isTrustedIP(r.RemoteAddr)`. `isTrustedIP` is false when `trustedIPs` is empty (`ipChecker == nil`), so every peer is untrusted unless `insecure` is true ([traefik@faa1eb59:pkg/middlewares/forwardedheaders/forwarded_header.go](https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/middlewares/forwardedheaders/forwarded_header.go)).

Official docs: `trustedIPs` is “the IPs or CIDR from where Traefik trusts the forwarded headers information (`X-Forwarded-*`)”; `insecure` is “always trust” and “only for tests purposes” ([EntryPoints v3.7](https://doc.traefik.io/traefik/v3.7/reference/install-configuration/entrypoints/)).

Extracts: [.sources/entrypoints-v3.7.md](.sources/entrypoints-v3.7.md), [.sources/security-decisions.md](.sources/security-decisions.md), [.sources/server_entrypoint_tcp.go.md](.sources/server_entrypoint_tcp.go.md), [.sources/forwarded_header.go.md](.sources/forwarded_header.go.md)

## Untrusted TCP peer: managed headers dropped, then Traefik fills empties

When `insecure` is false and `RemoteAddr` is not in `trustedIPs`, Traefik **deletes** every managed header (canonical `X-Forwarded-*`, `X-Real-Ip`, and underscore variants such as `X_Forwarded_For`) via `DeleteXForwardedHeaders`, then `rewrite` fills headers that are now empty ([traefik@faa1eb59:forwarded_header.go](https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/middlewares/forwardedheaders/forwarded_header.go), tests in [forwarded_header_test.go](https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/middlewares/forwardedheaders/forwarded_header_test.go)).

At plugin `ServeHTTP` time after that:

| Header | Result |
| --- | --- |
| `X-Forwarded-For` | Empty. `rewrite` only joins existing XFF values; it does not append `RemoteAddr`. The TCP peer is appended later, when Traefik proxies to the backend ([EntryPoints / IPAllowList](https://doc.traefik.io/traefik/v3.7/reference/routing-configuration/http/middlewares/ipallowlist/), [proxy.go](https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/proxy/httputil/proxy.go)). |
| `X-Real-Ip` | Set to the TCP peer IP when `net.SplitHostPort(RemoteAddr)` succeeds. |
| Other managed `X-Forwarded-*` | Dropped, then empty ones filled from the connection (`Proto`, `Port`, `Host`) plus `X-Forwarded-Server` = Traefik hostname. |
| RFC 7239 `Forwarded` | **Not** in the managed set. Left as the client sent it (`authority: source` from `XHeadersSet`; official docs only name `X-Forwarded-*`). |

They are not left as the client sent them. They are not rewritten in place. They are stripped, then Traefik writes connection-derived values into the empty slots.

Extracts: [.sources/forwarded_header.go.md](.sources/forwarded_header.go.md), [.sources/forwarded_header_test.go.md](.sources/forwarded_header_test.go.md), [.sources/proxy.go.md](.sources/proxy.go.md)

## Trusted TCP peer (or insecure): client values kept, empties filled

When `insecure` is true, or `RemoteAddr` is in `trustedIPs`, Traefik skips the delete. Incoming managed headers stay. `rewrite` still runs: if `X-Real-Ip` is empty it is set from the TCP peer; existing `X-Real-Ip` is kept; multiple `X-Forwarded-For` values are joined with `", "`; empty `X-Forwarded-Proto` / `Port` / `Host` are filled from the connection ([traefik@faa1eb59:forwarded_header.go](https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/middlewares/forwardedheaders/forwarded_header.go)).

`insecure: true` is the same keep-and-fill path for **any** peer ([EntryPoints v3.7](https://doc.traefik.io/traefik/v3.7/reference/install-configuration/entrypoints/), config comment “Trust all forwarded headers” in [entrypoints.go](https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/config/static/entrypoints.go)).

At plugin time, trusted XFF is still the client-supplied list. Traefik has not yet appended `RemoteAddr`. That append happens in the reverse proxy unless `notAppendXForwardedFor` is set (v3.7.11; not in the v3.5 field table) ([proxy.go](https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/proxy/httputil/proxy.go)).

Extracts: [.sources/forwarded_header.go.md](.sources/forwarded_header.go.md), [.sources/entrypoints.go.md](.sources/entrypoints.go.md), [.sources/proxy.go.md](.sources/proxy.go.md)

## RemoteAddr stays the TCP peer

`forwardedheaders` never assigns `req.RemoteAddr`. A plugin still sees the TCP peer (host:port) ([traefik@faa1eb59:forwarded_header.go](https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/middlewares/forwardedheaders/forwarded_header.go)).

A different entrypoint feature, `proxyProtocol.trustedIPs` / `insecure`, can replace the connection remote address before HTTP is parsed. Official docs: “The replacement of the remote client address will occur only for IP addresses listed in `trustedIPs`” ([EntryPoints v3.7](https://doc.traefik.io/traefik/v3.7/reference/install-configuration/entrypoints/), [server_entrypoint_tcp.go](https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/server/server_entrypoint_tcp.go)). That is PROXY protocol, not `forwardedHeaders`.

Extracts: [.sources/forwarded_header.go.md](.sources/forwarded_header.go.md), [.sources/server_entrypoint_tcp.go.md](.sources/server_entrypoint_tcp.go.md), [.sources/entrypoints-v3.7.md](.sources/entrypoints-v3.7.md)

## No precomputed client IP is passed into plugin ServeHTTP

Yaegi `New(ctx, next, config, name)` must return `http.Handler`. Traefik calls that handler with the ordinary `*http.Request`. There is no extra argument, request field, or Traefik-only header that carries a selected client hop ([traefik@faa1eb59:pkg/plugins/middlewareyaegi.go](https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/plugins/middlewareyaegi.go)).

What the plugin can read is the request after entrypoint sanitization:

- `req.RemoteAddr` — TCP peer (unless PROXY protocol rewrote the connection).
- `X-Real-Ip` — always filled from the TCP peer when empty after the trust decision; kept when a trusted (or insecure) peer sent it.
- `X-Forwarded-For` — empty if untrusted; client-supplied list if trusted/insecure; TCP peer not yet appended.

That is sanitization, not a selected client address. A plugin that needs one hop from a chain is reconstructing, starting from these already-sanitized values.

Extracts: [.sources/middlewareyaegi.go.md](.sources/middlewareyaegi.go.md), [.sources/forwarded_header.go.md](.sources/forwarded_header.go.md)

## ipStrategy.excludedIPs is Traefik’s own middleware rule, not a plugin input

Peer reconstruction used by Traefik middlewares (`ipAllowList`, `ipWhiteList`, `rateLimit` / `inFlightReq` `sourceCriterion`). It is not written back onto the request. `IPAllowList.ServeHTTP` calls `strategy.GetIP(req)` locally and either rejects or calls `next` with the same `req` ([traefik@faa1eb59:pkg/middlewares/ipallowlist/ip_allowlist.go](https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/middlewares/ipallowlist/ip_allowlist.go)). Plugins do not receive that result.

Rules ([IPAllowList v3.7](https://doc.traefik.io/traefik/v3.7/reference/routing-configuration/http/middlewares/ipallowlist/), [pkg/ip/strategy.go](https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/ip/strategy.go), [IPStrategy.Get](https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/config/dynamic/middlewares.go)):

- No strategy → `RemoteAddrStrategy` (TCP peer IP).
- `depth` > 0 → from the **right** of `X-Forwarded-For`; `depth` 1 is the rightmost hop; `excludedIPs` is ignored. Too deep → empty string.
- `excludedIPs` without depth → `PoolStrategy` walks XFF **right to left** (most recent first) and returns the first IP **not** in the excluded pool; all excluded → empty string.
- `depth` overrides `excludedIPs`.

Relation to `forwardedHeaders`: `ipStrategy` reads `X-Forwarded-For` **after** entrypoint sanitization. Official docs: passlisting happens before proxying, and the previous hop is appended to XFF only at the last stages of proxying, so that hop is not in XFF during the middleware ([IPAllowList v3.7](https://doc.traefik.io/traefik/v3.7/reference/routing-configuration/http/middlewares/ipallowlist/)). If the TCP peer was untrusted, XFF is empty at middleware time, so depth / excludedIPs yield empty; only `RemoteAddrStrategy` still has an IP.

Extracts: [.sources/ipallowlist-v3.7.md](.sources/ipallowlist-v3.7.md), [.sources/strategy.go.md](.sources/strategy.go.md), [.sources/middlewares.go.md](.sources/middlewares.go.md), [.sources/ip_allowlist.go.md](.sources/ip_allowlist.go.md)

## Authority

| Claim | Owner | Rank |
| --- | --- | --- |
| Trust decided once at entrypoint, before middleware | Security Decisions | official |
| `trustedIPs` / `insecure` mean trust `X-Forwarded-*` | EntryPoints v3.7 (same on v3.5) | official |
| `insecure` recommended only for tests | EntryPoints v3.7 | official |
| `notAppendXForwardedFor` / `connection` exist on v3.7.11, not in v3.5 field table | EntryPoints v3.7 vs v3.5 | official |
| `XForwarded` wraps the router switcher on every HTTP request | traefik@faa1eb59:server_entrypoint_tcp.go | source |
| Untrusted peer → delete managed headers, then rewrite empties | traefik@faa1eb59:forwarded_header.go | source |
| Trusted / insecure → keep client managed headers, fill empties | traefik@faa1eb59:forwarded_header.go | source |
| Empty `trustedIPs` and `insecure=false` → every peer untrusted | traefik@faa1eb59:forwarded_header.go | source |
| Managed set is `X-Forwarded-*` + `X-Real-Ip` (+ `_` variants); not RFC `Forwarded` | traefik@faa1eb59:forwarded_header.go `XHeadersSet` | source |
| `rewrite` does not set `RemoteAddr`; does not append XFF | traefik@faa1eb59:forwarded_header.go | source |
| XFF TCP-peer append happens at reverse-proxy time | traefik@faa1eb59:pkg/proxy/httputil/proxy.go | source |
| Previous hop not yet in XFF during middleware | IPAllowList v3.7 | official |
| Yaegi plugin is `New` → `http.Handler`; no extra client-IP argument | traefik@faa1eb59:middlewareyaegi.go | source |
| `excludedIPs` walks XFF right-to-left, first IP not in pool | traefik@faa1eb59:strategy.go + IPAllowList v3.7 | source / official |
| `depth` from the right; overrides `excludedIPs` | IPAllowList v3.7 + IPStrategy.Get | official / source |
| IPAllowList does not write `GetIP` onto the request | traefik@faa1eb59:ip_allowlist.go | source |
| PROXY protocol (not forwardedHeaders) may replace remote address | EntryPoints v3.7 + server_entrypoint_tcp.go | official / source |
