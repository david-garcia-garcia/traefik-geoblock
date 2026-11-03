# Requirement
IssueKey: 2026-11-03-pr95-rightmost-untrusted
Source: https://github.com/david-garcia-garcia/traefik-geoblock/pull/95 (head `9b701a6`, branch `feat/rightmost-untrusted-ip-strategy`, author JoschaP)
Base: `master` @ `6b5acaf` (PR is up to date with master)

## Problem

PR #95 adds `ipHeaderStrategy: RightmostUntrusted` with a `trustedProxies` list. The design is right (same rule as Traefik `ipStrategy.excludedIPs`): walk the hops from the right, skip trusted proxies, evaluate exactly one hop. The implementation does not deliver the guarantee the README promises ("a client cannot choose the country"):

1. **Dedup bypass (critical).** The strategy runs on the output of `GetRemoteIPs`, which drops repeated IPs and keeps the *first* occurrence. A client that repeats its own IP to the left of a forged one makes the forged hop the rightmost untrusted one.
2. **Only the first header line is read (high).** `req.Header.Get` returns the first field line. If the trusted proxy appends a new `X-Forwarded-For` line instead of extending the existing one, the plugin only sees the line the client wrote.
3. **Header concatenation order (medium).** Several `ipHeaders` exist so that one middleware can sit behind different external proxies (e.g. Cloudflare sets `CF-Connecting-IP`, a load balancer sets `X-Forwarded-For`). Only one of them is expected on a given request. The PR instead concatenates the hops of every header present, in config order, and walks that combined list. If a request carries more than one configured header, "rightmost" means "from the last configured header", not the end of the real forwarding chain. With the default `ipHeaders` (`x-forwarded-for`, `x-real-ip`) a request carrying both is decided by `X-Real-IP`.
4. **Config and observability gaps (low).** `trustedProxies` is silently ignored with other strategies; CIDR matching does not use the project's shared helper; trace logs lose the full chain.

Findings 1–3 come from reading the code. They were **not executed**: running the contributor's code was blocked in the review session. Finding 1 follows directly from `GetRemoteIPs` on `master`. Each finding below has a test that must prove it.

## Current (code)

- `pkg/geoblock/ip.go` `GetRemoteIPs` (master and PR): for each `ipHeaders` entry, `req.Header.Get(name)` (or `req.RemoteAddr` for `remoteAddress`), split on `,`, `cleanIPAddress`, then **dedupe keeping the first occurrence** (`seenIPs`). Existing tests rely on that dedup: `pkg/geoblock/zzz_plugin_ipheaders_test.go` `Deduplication_Preserves_First_Occurrence`, `RemoteAddress_Deduplication`.
- PR `pkg/geoblock/plugin.go` `ServeHTTP` (~L326): `remoteIPs := p.GetRemoteIPs(req)` then, for `RightmostUntrusted`, `remoteIPs = p.rightmostUntrusted(remoteIPs)`. `ipChain := p.traceIPChain(remoteIPs)` is computed **after** the filter.
- PR `pkg/geoblock/ip.go` `rightmostUntrusted`: walks right to left; an unparsable hop or a hop not in `p.trustedProxies` is returned as the only hop; all trusted → `nil` (then `banIfError` path in `blockFromHeader`).
- PR `pkg/geoblock/ip.go` `parseTrustedProxies` / `inNets`: own `[]*net.IPNet` + linear `Contains`. Plain address → /32 or /128.
- PR `pkg/geoblock/config.go` `Prepare`: accepts the new strategy; calls `parseTrustedProxies` for validation only. PR `plugin.go` `NewCore` parses again and stores the result.
- CIDR allow/block elsewhere uses `github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup` (`pkg/geoblock/ip_blocks.go` `loadIPBlockHelper`). Live spec `openspec/specs/std_go_iplookup_family-isolated/spec.md` requires family-isolated, longest-prefix CIDR membership through that helper.
- `pkg/geoblock/config.go` `CreateConfig`: default `IPHeaders` is `["x-forwarded-for", "x-real-ip"]`.
- PR also carries commit `c64168f` (README CrowdSec link to the fork). Its content is identical to the maintainer's local branch `docs/crowdsec-fork-link` (`966d80c`).

## Desired — what to fix and how

Priority order. R1–R3 block merge. R4–R6 should land in the same PR. R7 is PR hygiene.

### R1 (critical) — No dedup on the RightmostUntrusted path

**What.** `RightmostUntrusted` MUST evaluate the hop list exactly as received, in order, without removing repeated IPs.

**How.**
- Split collection from dedup in `pkg/geoblock/ip.go`:
  - `collectHops(req *http.Request) []string` — header iteration, split, `cleanIPAddress`, no dedup.
  - `GetRemoteIPs` = `dedupeFirst(collectHops(req))`. Behaviour for `CheckAll` / `CheckFirst` / `CheckFirstNonePrivate` stays byte-for-byte the same (existing dedup tests must pass unchanged).
- In `ServeHTTP`, for `RightmostUntrusted`: `hops := p.collectHops(req)`; `remoteIPs := p.rightmostUntrusted(hops)`. Do not call `GetRemoteIPs` on this path.
- Do not "fix" this by deduping keeping the *last* occurrence: walking the raw list is simpler and leaves no ordering edge case.

**Acceptance.**
- `X-Forwarded-For: 8.8.8.8, 1.1.1.1, 8.8.8.8`, `trustedProxies: [10.10.60.0/24]`, `blockedCountries: [US]` → 403, country `US` (today: `AU` / pass).
- `ipHeaders: [x-forwarded-for, remoteAddress]`, XFF `1.1.1.1`, `RemoteAddr` `1.1.1.1:5000`, nothing trusted → evaluated hop `1.1.1.1` (a repeated IP is harmless).
- Existing `Deduplication_Preserves_First_Occurrence` and `RemoteAddress_Deduplication` pass unchanged.

### R2 (high) — Read every field line of a header

**What.** On the `RightmostUntrusted` path, a header sent as several field lines MUST be read as one list: all lines, in order, each split on `,`.

**How.**
- In `collectHops`, use `req.Header.Values(name)` and iterate the lines in order (equivalent to joining them with `,`). `remoteAddress` is unchanged.
- Scope: required for `RightmostUntrusted`. Applying it to the other strategies changes which hops `CheckAll` evaluates. That is a behaviour change for existing users and is **out of scope** here (see Tensions). Pass a flag or use a separate code path so that the other strategies keep reading only the first line.

**Acceptance.**
- `req.Header.Add("X-Forwarded-For", "1.1.1.1")`, then `req.Header.Add("X-Forwarded-For", "8.8.8.8")`, `blockedCountries: [US]` → 403, country `US` (today: only `1.1.1.1` is read).
- Mixed: lines `"1.1.1.1, 2.2.2.2"` and `"8.8.8.8, 10.10.60.5"`, `trustedProxies: [10.10.60.0/24]` → evaluated hop `8.8.8.8`.

### R3 (medium) — Headers are alternatives: first present header wins, `remoteAddress` closes the chain

**What.** Several `ipHeaders` are kept to support several external proxies (one per entry path: Cloudflare, load balancer, CDN…). Only one is expected per request. "Rightmost" must be computed inside **one** forwarding chain. For `RightmostUntrusted`:

1. **Header selection.** The forwarding headers (every `ipHeaders` entry except `remoteAddress`) are alternatives, tried in config order. The first one present on the request (at least one non-empty hop after R2) is the selected header. All later headers are ignored for that request. They are never concatenated.
2. **Chain.** chain = hops of the selected header (left to right) + `RemoteAddr` as the last hop, when `remoteAddress` is listed.
3. **Selection.** `rightmostUntrusted(chain)` as today (after R1 and R4).
4. **`remoteAddress` position.** It MUST be the last `ipHeaders` entry; otherwise creation fails. It always closes the chain, so a different position would suggest an order that does not exist.
5. **No forwarding header present.** The chain is just `RemoteAddr` (if listed), else empty → existing "no client IP" path (`banIfError`).

Consequence (the hardening this gives): when `remoteAddress` is listed and the direct peer is **not** in `trustedProxies`, that peer is the rightmost untrusted hop. The request did not come through any of our proxies, so every header (forged or not) is ignored. That is the same idea as Traefik entrypoint `forwardedHeaders.trustedIPs`.

**How.**
- In `pkg/geoblock/ip.go`, a function used only by this strategy, e.g. `rightmostChain(req) []string`: iterate `p.ipHeaders` skipping `remoteAddress`; for the first header with hops (via `collectHops` for one header, R1 + R2), stop; then append `cleanIPAddress(req.RemoteAddr)` if `remoteAddress` is configured.
- `Prepare` (`pkg/geoblock/config.go`): when the strategy is `RightmostUntrusted` and `remoteAddress` is present but not last → error, e.g. `"<name>: RightmostUntrusted needs remoteAddress as the last ipHeaders entry; got [remoteAddress x-forwarded-for]"`.
- `NewCore`: when the strategy is `RightmostUntrusted` and `remoteAddress` is **not** listed → one WARN at creation: headers are then trusted without checking who sent them. Do not reject: deployments behind a single proxy that always overwrites the header are valid.
- The default `ipHeaders` (`x-forwarded-for`, `x-real-ip`) stays valid with this strategy: `X-Forwarded-For` when present, else `X-Real-IP`.
- The other strategies keep concatenating as today (out of scope).

**Residual risk to document (not fixed here).** Header selection is decided by the request. If a client can get a configured header through a trusted proxy that does not overwrite or strip it (e.g. it sends `CF-Connecting-IP` through the load balancer when the load balancer's own header is `X-Forwarded-For`, and `cf-connecting-ip` comes first in the list), the client picks the header and its value. Requirements for the operator: every proxy must overwrite or strip every configured header it does not set, or `ipHeaders` must only list headers that cannot arrive forged. Binding each header to the proxies allowed to set it (per-header trusted sources) would close this. It is a separate feature (see Out of scope).

**Acceptance.**
- `[cf-connecting-ip, x-forwarded-for, remoteAddress]`, only `X-Forwarded-For: 8.8.8.8` present, RemoteAddr trusted → evaluated `8.8.8.8`.
- Same config, only `CF-Connecting-IP: 8.8.8.8` present, RemoteAddr trusted → evaluated `8.8.8.8`.
- Same config, both present (`CF-Connecting-IP: 1.1.1.1`, `X-Forwarded-For: 8.8.8.8`) → only `CF-Connecting-IP` used → `1.1.1.1`. This proves "first present wins, no concatenation".
- Same config, `X-Forwarded-For: 1.1.1.1` forged, RemoteAddr `8.8.8.8:5000` **not** trusted → evaluated `8.8.8.8` (headers ignored).
- `[x-forwarded-for, x-real-ip]` (default) → accepted; with both headers present, `X-Forwarded-For` decides.
- `[remoteAddress, x-forwarded-for]` → creation error.
- `[x-forwarded-for]` without `remoteAddress` → accepted, one WARN at creation.
- No configured forwarding header present, `[x-forwarded-for, remoteAddress]` → evaluated RemoteAddr (unless trusted → `banIfError` path).
- Other strategies: no new validation; existing configs keep loading.

### R4 (low) — `trustedProxies` validation and matching

**What.**
1. `trustedProxies` set (non-empty after trimming) with a strategy other than `RightmostUntrusted` MUST fail creation. A security setting that is silently ignored misleads the operator. The field is new, so nobody is broken.
2. Membership MUST use the shared `iplookup.Helper`, like `allowedIPBlocks` / `blockedIPBlocks`. That gives the same CIDR semantics, the family isolation required by `std_go_iplookup_family-isolated` and longest-prefix lookup.
3. Parse once.

**How.**
- Replace `parseTrustedProxies` + `inNets` with a builder that returns `*iplookup.Helper`. A plain address is turned into `addr/32` (IPv4) or `addr/128` (IPv6) before `AddCIDR`. An invalid entry returns an error naming the entry.
- `NewCore` builds the helper and stores it on `Plugin`. `Prepare` keeps only cheap checks (R3, R4.1) and, if early syntax validation is wanted, calls the same builder and discards the result. There must be no second parser.
- `rightmostUntrusted` parses each hop with `net.ParseIP` once and calls `helper.Contains(ip)`. A `Contains` error is treated as "not trusted" (fail closed: the hop is evaluated, not skipped).
- Empty `trustedProxies` with `RightmostUntrusted` stays valid. With a single load balancer whose address is only in `RemoteAddr`, the rightmost XFF hop is the client, so an empty list is a correct configuration.

**Acceptance.**
- `trustedProxies: [10.0.0.0/8]` + `CheckAll` → creation error.
- `trustedProxies: ["0.0.0.0/0"]` and an IPv6 client hop → the IPv6 hop is **not** trusted (family isolation).
- `trustedProxies: ["2001:db8::1"]` → hop `2001:db8::1` skipped; `2001:db8::2` evaluated.
- `trustedProxies: ["not-an-ip"]`, `["10.0.0.0/33"]` → creation error naming the entry.

### R5 (low) — Trace and error logs keep the full chain

**What.** With `RightmostUntrusted`, trace logs and `request check failed` MUST show the full collected chain, plus the hop that was selected. Debugging `trustedProxies` is impossible without both.

**How.**
- In `ServeHTTP`, compute `ipChain` from `hops` (pre-filter) on this path, not from the filtered slice.
- Add a `selected_ip` attribute (or reuse the existing `ip` attribute) where the selection is logged. When every hop is trusted, log at DEBUG that no untrusted hop was found, with the chain.
- `logLookupError` falls back to `strings.Join(remoteIPs, ", ")` when `ipChain` is empty. On this path, pass the full chain so that the fallback does not print a single hop.

**Acceptance.** A unit test with a captured logger at trace level: the chain attribute contains every hop and the selected-hop attribute contains the evaluated one.

### R6 — Documentation

**What / How (README.md, IP headers section).**
- State the R3 rule (headers are alternatives, first present wins, `remoteAddress` closes the chain) and show the recommended config for several external proxies:
  ```yaml
  ipHeaders:
    - cf-connecting-ip     # requests that come through Cloudflare
    - x-forwarded-for      # requests that come through the load balancer
    - remoteAddress        # always last: the direct peer
  ipHeaderStrategy: RightmostUntrusted
  trustedProxies:
    - 10.10.60.0/24        # load balancer
    - 173.245.48.0/20      # Cloudflare range (example; use the published list)
  ```
- Explain why `remoteAddress` must be last and what it protects: a direct peer outside `trustedProxies` makes every header ignored.
- Document the R3 residual risk: each proxy must overwrite or strip the configured headers it does not set.
- Explain the interplay with Traefik entrypoint `forwardedHeaders.trustedIPs`. Traefik drops forwarded headers that come from peers it does not trust, so `X-Forwarded-For` may be empty at middleware time; with `remoteAddress` last there is still a hop to evaluate. **Verify** this behaviour against the Traefik docs for the supported version before writing it; do not state it from memory.
- State that `trustedProxies` is rejected with the other strategies (R4.1).
- Keep the existing claim "same rule as Traefik `ipStrategy.excludedIPs`" only if R1–R3 are in: today the dedup makes it false.

### R7 — PR hygiene

- Commit `c64168f` (CrowdSec fork link) is the maintainer's `docs/crowdsec-fork-link` change. Either merge that branch first and rebase the PR, or drop the commit from the PR. It must not land as part of this feature.
- The PR body reports `TestNew_UnreclaimedHashDisposesAfterGrace` as flaky on `master` (fixed 80 ms sleep). Not verified here. Track it in a separate issue; it is not part of this PR.

## Test plan (all in `pkg/geoblock/zzz_plugin_rightmost_untrusted_test.go` unless noted)

| # | Case | Expect |
|---|---|---|
| T1 | XFF `8.8.8.8, 1.1.1.1, 8.8.8.8`, trusted `10.10.60.0/24`, block US | 403, `US` (R1) |
| T2 | Two XFF lines `1.1.1.1` / `8.8.8.8`, block US | 403, `US` (R2) |
| T3 | Lines `1.1.1.1, 2.2.2.2` / `8.8.8.8, 10.10.60.5` | evaluated `8.8.8.8` (R2) |
| T4 | `[x-forwarded-for, remoteAddress]`, XFF `8.8.8.8`, RemoteAddr `10.10.60.5:443` trusted | evaluated `8.8.8.8` |
| T5 | `[x-forwarded-for, remoteAddress]`, XFF `1.1.1.1`, RemoteAddr `1.1.1.1:5000` | evaluated `1.1.1.1` |
| T6 | `ipHeaders` alternatives and `remoteAddress` position (R3 acceptance list) | as listed in R3 |
| T6b | `[cf-connecting-ip, x-forwarded-for, remoteAddress]`, both headers present | only `CF-Connecting-IP` used, no concatenation |
| T6c | Forged XFF, RemoteAddr not trusted | RemoteAddr evaluated, headers ignored |
| T7 | `trustedProxies` + `CheckAll` | creation error (R4.1) |
| T8 | `0.0.0.0/0` trusted, IPv6 client hop | IPv6 hop evaluated (R4.2) |
| T9 | Single IPv6 address entry | exact match only |
| T10 | Invalid entries (`not-an-ip`, `/33`) | creation error naming the entry |
| T11 | All hops trusted, `banIfError` true / false | block / pass (already in PR; keep) |
| T12 | Trace log attributes (R5) | full chain + selected hop |
| T13 | `zzz_plugin_ipheaders_test.go` existing dedup tests | unchanged, pass |

`go test ./...` green in CI. If possible, one integration case through `Test-Integration.ps1` / docker-compose with a real `X-Forwarded-For` from a proxy in front of Traefik.

## Affected

- `pkg/geoblock/ip.go` — `GetRemoteIPs` split into `collectHops` + dedup; `rightmostUntrusted`; trusted-proxy helper builder (replaces `parseTrustedProxies` / `inNets`)
- `pkg/geoblock/plugin.go` — `Plugin.trustedProxies` type; `NewCore`; `ServeHTTP` path for the strategy; `logLookupError` call site
- `pkg/geoblock/config.go` — `Prepare` validation (R3, R4.1)
- `pkg/geoblock/zzz_plugin_rightmost_untrusted_test.go` — new cases
- `README.md` — IP headers section

## Out of scope

- Changing dedup or multi-line reading for `CheckAll` / `CheckFirst` / `CheckFirstNonePrivate`.
- Parsing the RFC 7239 `Forwarded` header.
- Changing `banIfError` / `defaultAllow` defaults.
- Changing how an unparsable hop enriches (`XX`) for the other strategies.
- Per-header trusted sources (honour header H only when the request came from proxy set S). This would close the R3 residual risk; it is a follow-up feature.

## Unknowns

- Does the maintainer want an openspec spec for IP header strategies? None exists today (`openspec/specs` has none for `ipHeaders`). If yes, add `core_geoblock_ip-header-strategy` with R1–R4 as scenarios.
- Exact Traefik behaviour for forwarded headers from untrusted peers (R6) on the Traefik versions this plugin supports.
- Which real proxies in front of Traefik emit `X-Forwarded-For` as separate field lines (R2). The fix does not depend on the answer, but the README example should not name a product without checking.

## Tensions

- **Multi-line reading for all strategies.** For `CheckAll` it closes the same blind spot, but it adds hops and can block traffic that passes today. Keep it out of this PR and decide separately.
- **Unparsable rightmost hop.** The PR returns it as the client hop, so it enriches as `XX` and `defaultAllow` decides. The alternative is to treat it as "no client IP" (`banIfError`). An unparsable hop on the right can only come from a trusted proxy or from a direct-exposure misconfiguration, where the client already controls the whole header. So it adds no new bypass. Keep the PR behaviour; document it.
- **Header selection driven by the request (R3).** "First present header wins" supports several external proxies with one middleware, as intended. The cost: a client that can get a configured header through a proxy that does not strip it chooses the header. Listing `remoteAddress` last removes the direct-to-Traefik case. The through-a-trusted-proxy case remains an operator responsibility until per-header trusted sources exist.
