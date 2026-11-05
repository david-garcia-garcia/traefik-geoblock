## In one sentence
Adds an opt-in `ipHeaderStrategy: RightmostUntrusted` so that, behind load balancers or CDNs, the plugin geo-checks the real client instead of an IP the client wrote into a header.

## Why
The existing strategies (`CheckAll`, `CheckFirst`, `CheckFirstNonePrivate`) merge every configured header into one list and drop repeated IPs. A client can use that to choose which country gets checked:

| What arrives | What the old strategies see | Real client |
| --- | --- | --- |
| `X-Forwarded-For: 8.8.8.8, 1.1.1.1, 8.8.8.8` (client forged the first two, proxy added the last) | `8.8.8.8, 1.1.1.1`: dedup removes the last entry, so the forged `1.1.1.1` is now the last hop | `8.8.8.8`, the rightmost hop |
| A second `X-Forwarded-For` line | nothing (only the first line is read) | on the second line |
| `X-Real-IP` and `X-Forwarded-For` together | both, merged into one list | only the header your proxy sets |

Priority: P2. Operators behind proxies have no safe way to select the client IP today.

## How the new strategy picks the client IP
1. **Choose one header.** Forwarding headers in `ipHeaders` are tried in the order you list them. The first one that contains an address wins. Absent or empty headers are skipped. Headers are never merged.
2. **Add the direct peer.** If `remoteAddress` is listed (it must be last), the TCP peer is appended as the last hop.
3. **Walk from the right.** Skip every hop that is in `trustedProxies`. The first hop that is not trusted is the only one checked.
4. **Edge cases.**
   - Every hop is trusted (for example a load balancer health check): the leftmost hop, where the request started, is checked. A private LB address follows `allowPrivate`.
   - The checked hop is not an IP (`garbage`): blocked when `banIfError: true`; otherwise `defaultAllow` decides.
   - No hop at all: same as today (`banIfError`).

```yaml
ipHeaders:
  - cf-connecting-ip     # requests that come through Cloudflare
  - x-forwarded-for      # requests that come through the load balancer
  - remoteAddress        # always last: the direct peer
ipHeaderStrategy: RightmostUntrusted
trustedProxies:
  - 10.10.60.0/24        # load balancer
  - 173.245.48.0/20      # Cloudflare range (example)
```

## Changes after code review (2026-11-05)
| # | Problem found | What changed |
| --- | --- | --- |
| 1 | A `trustedProxies` or `blockedIPBlocks` entry like `::ffff:10.0.0.0/104` crashed plugin creation (bug in `iplookup` v1.0.3). | `traefik-middleware-utilities` bumped from v1.0.3 to v1.0.8, which fixes it. |
| 2 | A bare `::ffff:10.10.60.5` in `trustedProxies` trusted the whole IPv6 range `::/32` and not the load balancer. | `trustedProxies` now uses the new `iplookup.AddEntry`, which stores a bare address as its exact host. |
| 3 | With `banIfError: false`, an evaluated hop that is not an IP passed even with `defaultAllow: false`. | `defaultAllow` now decides that hop. |
| 4 | Requests coming straight from a trusted proxy (health checks) had no hop left: a WARN per request and a 403 with the default `banIfError: true`. | The leftmost hop is checked instead; health checks from a private LB pass with `allowPrivate`. No warning. |
| 5 | Question: should an empty header stop the search? | No. Confirmed rule: the first **non-empty** header in the order you configure wins. Now covered by tests and stated in the spec. |
| 6 | The default `CheckAll` path allocated 3–4 times per request after the refactor. | Back to one pass with inline dedup: one allocation for a single hop (guarded by a test). |
| 7 | Hop parsing was duplicated between the two paths. | One shared `appendHops` loop. |
| 8 | Every request joined the full chain into a string and locked the trusted-proxy helper even when the list was empty. | The chain is joined only at trace level; no helper when `trustedProxies` is empty; `iplookup` now uses a read lock. |

## Who is affected
- **Operators:** nothing changes unless you set `ipHeaderStrategy: RightmostUntrusted`. `trustedProxies` with another strategy fails at startup. Put `remoteAddress` last. Each proxy must overwrite or strip the forwarding headers it does not set, or a client can supply an earlier header.
- **Developers:** `Config.TrustedProxies` and `IPHeaderStrategyRightmostUntrusted` are new public names. `RightmostUntrusted` must not call `GetRemoteIPs`.
- **End users:** with the new strategy, geo allow/block follows the real client, not a forged IP.

## Before merging
- [x] `traefik-middleware-utilities` PR #102 (adds `AddEntry`) merged and tagged `v1.0.8`.
- [x] This branch uses `v1.0.8` (vendor refreshed, yaegi overlay for `maxminddb` re-applied).
- [ ] CI green on the new head.

## Verification
| Check | Result |
| --- | --- |
| `go test ./...` (local, 2026-11-05) | passed |
| New tests run against the previous code | 9 behaviour tests fail before the fix, pass after |
| `golangci-lint run ./...` (local) | no findings |
| CI on the new head | pending |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/97 |

## Specs
- [core_geoblock_plugin_ip-header-strategy](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-fix-rightmost-untrusted/openspec/specs/core_geoblock_plugin_ip-header-strategy/spec.md) — added (updated after review: empty headers, all-trusted chain, unparsable hop, IPv4-mapped entries)
- [Change proposal](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-fix-rightmost-untrusted/openspec/changes/archive/2026-11-03-fix-rightmost-untrusted/proposal.md)

## Deviations from the ask
- When every hop is trusted, the original spec sent the request to the `banIfError` path. That broke load balancer health checks, so the leftmost hop, where the request started, is now checked instead. It is a real hop, not an invented one, and it goes through the normal rules (`allowPrivate`, CIDR lists, country).

## Follow-up issues
- Binding each header to the proxies allowed to set it (for example, accepting `CF-Connecting-IP` only from Cloudflare ranges) is not implemented. Today this relies on each proxy stripping headers it does not set.
