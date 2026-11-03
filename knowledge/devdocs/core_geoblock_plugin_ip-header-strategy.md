# IP header strategy

## Language

**Hop**:
One cleaned IP token from an `ipHeaders` entry: `cleanIPAddress` on a comma-split header field, or on `req.RemoteAddr` when the entry is `remoteAddress`.
_Avoid_: client IP (that is the hop the strategy selected)

**Chain**:
The ordered hop list one strategy evaluates on a request.
_Avoid_: concatenating every present `ipHeaders` entry for `RightmostUntrusted`

## Overview

`pkg/geoblock/ip.go` owns the client-address fact. `ServeHTTP` consumes that owner's slice; enrich / `blockFromHeader` / `skipIP` do not re-parse headers. `CheckAll` / `CheckFirst` / `CheckFirstNonePrivate` still go through `GetRemoteIPs`. `RightmostUntrusted` goes through `rightmostChain` then `rightmostUntrusted`.

## How to use

- Call `GetRemoteIPs` only for `CheckAll` / `CheckFirst` / `CheckFirstNonePrivate`. That path is `dedupeFirst(collectHops)` with first-line `Header.Get` so existing dedup tests stay byte-for-byte.
- For `RightmostUntrusted`, call `clientHops` (or `rightmostChain` then `rightmostUntrusted`). Do not call `GetRemoteIPs` on that path. Do not copy Traefik `ipStrategy.excludedIPs`. `clientHops` joins the pre-filter chain for error logs even when trace is off.
- `collectHops` reads one header or the whole `ipHeaders` list, splits on `,`, runs `cleanIPAddress`, and does not dedupe. Pass all field lines (`Header.Values`) only on the `RightmostUntrusted` path.
- `rightmostChain`: forwarding `ipHeaders` (every entry except `remoteAddress`) are alternatives in config order. Take the first header with at least one hop. Then append `cleanIPAddress(req.RemoteAddr)` when `remoteAddress` is listed. Never concatenate two forwarding headers.
- Trusted-proxy membership: a dedicated builder onto `iplookup.Helper` (plain address → `/32` or `/128`, then `AddCIDR`). Do not reuse `loadIPBlockHelper` (directory lists). `Contains` error → hop is not trusted.
- Empty `GetRemoteIPs` / empty `rightmostUntrusted` result is still the request-mode empty-hop path (`banIfError`). Do not invent a hop.

## Pattern snippet

```go
remoteIPs, ipChain := p.clientHops(req)
```

## Key files

- `pkg/geoblock/ip.go` — `collectHops`, `GetRemoteIPs`, `rightmostChain`, `rightmostUntrusted`, trusted-proxy builder
- `pkg/geoblock/plugin.go` — `ServeHTTP` / `clientHops` strategy branch; `Plugin` helper field
- `pkg/geoblock/config.go` — `Prepare` (`RightmostUntrusted`, `remoteAddress` last, `trustedProxies` only with that strategy)
- `pkg/geoblock/zzz_plugin_ipheaders_test.go` — existing first-occurrence dedup
- `pkg/geoblock/zzz_plugin_rightmost_untrusted_test.go` — RightmostUntrusted cases

## Gotchas

- `Header.Get` is the first field line. Traefik joins managed XFF lines on the trusted path; `Values` still matters for headers Traefik does not manage and for tests that skip Traefik.
- A direct peer not in `trustedProxies` makes `remoteAddress` the rightmost untrusted hop, so every header is ignored.
- `skipIP` stays `CheckFirst` / `CheckFirstNonePrivate` only. A one-hop filtered slice needs no third branch.
- Callers of `iplookup.Helper.Contains` pass the hop `net.IP` this owner already chose. Do not re-parse the request to rebuild that address.
