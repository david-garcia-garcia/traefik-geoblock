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

- Call `GetRemoteIPs` only for `CheckAll` / `CheckFirst` / `CheckFirstNonePrivate`. It reads the first field line (`Header.Get`) of every entry and dedupes inline in `appendHops` (first occurrence wins; the map is made on the second hop). One hop is one allocation; keep it that way.
- For `RightmostUntrusted`, call `clientHops` (or `rightmostChain` then `rightmostUntrusted`). Do not call `GetRemoteIPs` on that path. Do not copy Traefik `ipStrategy.excludedIPs`. `clientHops` joins the chain only when trace is on, like the other strategies; error logs fall back to the evaluated hop.
- `appendHops` is the one comma-split + `cleanIPAddress` loop for both paths. Pass a nil `seen` to keep repeated hops.
- `rightmostChain`: forwarding `ipHeaders` (every entry except `remoteAddress`) are alternatives in config order. Take the first header with at least one hop: absent, empty, and comma-only headers are skipped. Read every field line (`Header.Values`) of that header. Then append `req.RemoteAddr` when `remoteAddress` is listed (always last). Never concatenate two forwarding headers.
- `rightmostUntrusted`: from the right, the first hop not in `trustedProxies`. All hops trusted → the leftmost hop (the request's origin), so LB health checks follow `allowPrivate` instead of the empty-hop path. Empty only when the chain is empty. The result aliases the chain slice.
- Trusted-proxy membership: `newTrustedProxyHelper` calls `iplookup.Helper.AddEntry` (CIDR or bare address; a bare `::ffff:a.b.c.d` is the IPv4 `/32`). Do not append `/32` or `/128` here. The helper is nil when no entry is set. Do not reuse `loadIPBlockHelper` (directory lists). `Contains` error → hop is not trusted.
- `blockFromHeader`: a `decide` error on the `RightmostUntrusted` hop with `banIfError: false` is decided by `defaultAllow` (`PhaseDefaultAllow`), never skipped. The other strategies still skip that hop.
- An empty chain is still the request-mode empty-hop path (`banIfError`). Do not invent a hop.

## Pattern snippet

```go
remoteIPs, ipChain := p.clientHops(req)
```

## Key files

- `pkg/geoblock/ip.go` — `appendHops`, `GetRemoteIPs`, `rightmostChain`, `rightmostUntrusted`, `newTrustedProxyHelper`
- `pkg/geoblock/plugin.go` — `ServeHTTP` / `clientHops` strategy branch; `Plugin` helper field
- `pkg/geoblock/config.go` — `Prepare` (`RightmostUntrusted`, `remoteAddress` last, `trustedProxies` only with that strategy)
- `pkg/geoblock/zzz_plugin_ipheaders_test.go` — existing first-occurrence dedup
- `pkg/geoblock/zzz_plugin_rightmost_untrusted_test.go` — RightmostUntrusted cases

## Gotchas

- `Header.Get` is the first field line. Traefik joins managed XFF lines on the trusted path; `Values` still matters for headers Traefik does not manage and for tests that skip Traefik.
- A direct peer not in `trustedProxies` makes `remoteAddress` the rightmost untrusted hop, so every header is ignored.
- `skipIP` stays `CheckFirst` / `CheckFirstNonePrivate` only. A one-hop filtered slice needs no third branch.
- Callers of `iplookup.Helper.Contains` pass the hop `net.IP` this owner already chose. Do not re-parse the request to rebuild that address.
