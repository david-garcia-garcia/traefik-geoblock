# Explore

## Concepts

```
  Traefik entrypoint                 this plugin (Yaegi ServeHTTP)
  forwardedHeaders.trustedIPs        GetRemoteIPs ──► enrich / blockFromHeader
         │                              │                 │
         │ sanitizes header bytes       │ hop list        skipIP + decide
         ▼                              ▼
  *http.Request { Header, RemoteAddr }
         │
         └─ no precomputed client IP field
```

Units this change would touch:

| Unit | Path | Job |
| --- | --- | --- |
| Hop collector | `pkg/geoblock/ip.go` `GetRemoteIPs` | Read `ipHeaders` (Header.Get or `req.RemoteAddr`), split on `,`, `cleanIPAddress`, **dedupe first occurrence**. |
| Strategy gate | `pkg/geoblock/config.go` `Prepare` | Accept only `CheckAll` / `CheckFirst` / `CheckFirstNonePrivate`. |
| Request path | `pkg/geoblock/plugin.go` `ServeHTTP` | `remoteIPs := p.GetRemoteIPs(req)` then `traceIPChain` / enrich / block. No `RightmostUntrusted` branch. |
| Hop skip | `pkg/geoblock/plugin.go` `skipIP` | `CheckFirst` / `CheckFirstNonePrivate` only; any other strategy evaluates every remaining hop. |
| CIDR helper | `pkg/geoblock/ip_blocks.go` `loadIPBlockHelper` | Static + directory CIDRs onto `iplookup.Helper` for allow/block lists. Not hop selection. |
| Yaegi Config | `plugin.go` `type Config = geoblock.Config` | Traefik decodes onto the same struct. |
| README Client IP | `README.md` | Documents concatenate + first-occurrence dedup. |
| Tests | `pkg/geoblock/zzz_plugin_ipheaders_test.go` | Dedup and header-order pins. New file `zzz_plugin_rightmost_untrusted_test.go` on PR #95 only. |

PR #95 (`9b701a6`) adds `rightmostUntrusted` + `parseTrustedProxies` / `inNets` (`[]*net.IPNet`) and filters **after** `GetRemoteIPs`. Those symbols are `not found` on this worktree (`origin/master` lineage). Implement starts from dest.

How they connect: entrypoint may rewrite forwarded headers before the plugin runs; the plugin still reconstructs the client hop from `Header` + `RemoteAddr`. `ServeHTTP` owns when that slice is built. `skipIP` / `decide` consume the slice. `iplookup.Helper` already owns CIDR membership for allow/block (`openspec/specs/std_go_iplookup_family-isolated`).

Call sites that matter (`bounded` enumeration):

- `GetRemoteIPs`: 1 production (`pkg/geoblock/plugin.go` `ServeHTTP`), 2 tests (`zzz_plugin_ipheaders_test.go` HeaderOrder + SyntheticRemoteAddress). Roots searched: `pkg/`, module-root `plugin.go`. `vendor/github.com/david-garcia-garcia/traefik-middleware-utilities` — `not found`.
- `Prepare` strategy enum: 1 production (`config.go`), tests via `newTestPlugin` → `Prepare` (including `TestIPHeaderStrategy_InvalidStrategy`). Root `New` is the other production caller of `Prepare`.
- `skipIP`: 2 production (`enrich`, `blockFromHeader`). After a one-hop filter it no-ops.
- `loadIPBlockHelper`: 2 `NewCore` calls (allowed / blocked). Do not reuse for `trustedProxies` (directory walk is a different job).

Reproduce: **reproduced** on throwaway worktree `d:\repositories\traefik-geoblock-tmp-pr95-repro` detached at PR #95 head `9b701a6` (ticket worktree stayed `2026-11-03-fix-rightmost-untrusted`). Path: `go test ./pkg/geoblock -run TestExplore_PR95ClaimedFailures -v -count=1` (throwaway test, not committed).

| Claim | Observed on `9b701a6` | Desired (requirement) |
| --- | --- | --- |
| F1 dedup bypass. XFF `8.8.8.8, 1.1.1.1, 8.8.8.8`, trusted `10.10.60.0/24`, block US | hops=`[8.8.8.8 1.1.1.1]` (second `8.8.8.8` dropped), status **418**, country **AU** | 403, `US` |
| F2 first line only. `Add("1.1.1.1")` then `Add("8.8.8.8")` | hops=`[1.1.1.1]`, `Values`=`[1.1.1.1 8.8.8.8]`, status **418**, country **AU** | 403, `US` |
| F3 concatenate. default-like `[x-forwarded-for, x-real-ip]`, XFF `8.8.8.8`, Real-IP `1.1.1.1` | hops=`[8.8.8.8 1.1.1.1]`, status **418**, country **AU** (rightmost of the combined list) | first present header wins → XFF `8.8.8.8` → 403, `US` |

Contributor `TestIPHeaderStrategy_RightmostUntrusted` **PASS** on the same tree (happy path only; these three cases are absent). 418 is `noopHandler` (`StatusTeapot`) = allowed.

Outside facts used:

- Research: `knowledge/research/ext_traefik_forwardedheaders_trustedips/` (Traefik `v3.7.11` / `traefik@faa1eb59`). Untrusted peer: managed `X-Forwarded-*` / `X-Real-Ip` deleted then empties filled — XFF stays empty at middleware; `X-Real-Ip` becomes the TCP peer. Trusted/`insecure`: client values kept, multiple XFF lines joined with `", "`. `req.RemoteAddr` is still the TCP peer. Yaegi `ServeHTTP` gets ordinary `*http.Request`; Traefik `ipStrategy.excludedIPs` is not written onto it.
- In-tree: `knowledge/devdocs/core_geoblock_test-harness.md` (forwardedHeaders must stay on so the plugin sees `X-Real-IP`); `docker-compose.yml` sets both `trustedIPs` and `insecure=true` (the keep-all-peers path; does not demonstrate the untrusted delete). Compose image `traefik:v3.7.11`. README requires Traefik v3.5.0+.
- In-tree archive: `openspec/changes/archive/2026-09-14-bin-rwmutex-published-handle/design.md` — `GetRemoteIPs` remains the client-address owner.

## Decisions

- Seam: implement from dest. Add `RightmostUntrusted` + `trustedProxies` here; do not patch the contributor branch in place.
- Split hop collection: `collectHops` (no dedup; `Values` on the RightmostUntrusted path) and `GetRemoteIPs` = `dedupeFirst(collectHops)` with first-line `Get` so CheckAll / CheckFirst / CheckFirstNonePrivate stay byte-for-byte. R1 names this split.
- `ServeHTTP`: for `RightmostUntrusted` only, `hops := rightmostChain(req)` then `remoteIPs := rightmostUntrusted(hops)`. Do not call `GetRemoteIPs` on that path.
- R3 header rule (updated requirement): forwarding `ipHeaders` are **alternatives**, config order, first present (non-empty hops after R2) wins, never concatenated. `remoteAddress` MUST be last when listed and always closes the chain. Default `[x-forwarded-for, x-real-ip]` stays valid (XFF if present, else Real-IP). Missing `remoteAddress` is one WARN at creation, not a reject.
- Trusted-proxy membership: `iplookup.Helper` (same family isolation as allow/block). New builder: plain address → `/32` or `/128`, then `AddCIDR`. Do not keep `parseTrustedProxies` / `inNets`. Do not reuse `loadIPBlockHelper` (directory lists).
- `skipIP` stays CheckFirst / CheckFirstNonePrivate only; a one-hop filtered slice needs no third branch.
- Unparsable rightmost hop: keep PR behaviour (evaluate as the client hop → `XX` / `defaultAllow`). Requirement Tensions.
- Trace: `ipChain` from the pre-filter chain; log the selected hop (R5).
- README (R6, sourced): untrusted TCP peer → Traefik deletes managed `X-Forwarded-*` / `X-Real-Ip` then fills empties, so **XFF is empty at middleware**; `X-Real-Ip` is the TCP peer; `RemoteAddr` is still the TCP peer. With `remoteAddress` last there is still a hop. Default `[x-forwarded-for, x-real-ip]` without `remoteAddress` still has `X-Real-Ip`. This product’s compose sets `insecure=true`, so it does not show the delete path. Traefik already joins multiple XFF field lines with `", "` on the trusted path; R2 `Values` still matters for headers Traefik does not manage (`cf-connecting-ip`) and for tests that skip Traefik.
- README: document R3 alternatives + `remoteAddress` last + residual risk (operator must strip unused configured headers). Do not name a proxy product for multi-line XFF.
- Live contract: **no live contract** for IP header strategies (`openspec/specs` has none for `ipHeaders`). Neighbor live specs: `std_go_iplookup_family-isolated` (trustedProxies `Contains`); `core_geoblock_plugin_request-mode` (empty hop / `banIfError`, country from `countryHeader`). Propose adds `core_geoblock_ip-header-strategy` (R1–R4). Neighbor leftover change folder `openspec/changes/policy-guards-and-seed-fallback` is not this ticket.

Rejected:

- Dedup keeping the **last** occurrence — R1: walk the raw list.
- Concatenate every present `ipHeaders` entry for RightmostUntrusted — R3: alternatives, first present wins.
- Apply `Header.Values` / drop-dedup to CheckAll / CheckFirst / CheckFirstNonePrivate — Out of scope.
- Per-header trusted sources — Out of scope (R3 residual risk).
- Treat unparsable rightmost hop as “no client IP” — Tensions: keep PR.
- Reuse Traefik `ipStrategy.excludedIPs` output — not passed to Yaegi `ServeHTTP`.
- Reuse `loadIPBlockHelper` for `trustedProxies` — directory job, different owner.
- Cherry-pick PR #95 as-is — dest already has CrowdSec `c64168f`; R1–R3 are missing on that head.

## Open questions

- Q: Who already owns the client-address fact this change would reconstruct?
  Rank: bounded asked — existing `GetRemoteIPs` is the hop-list owner with enumerated callers (1 production `ServeHTTP`, 2 tests in `zzz_plugin_ipheaders_test.go`; roots `pkg/`, `plugin.go`); R1 names the collect/dedup split; R3 names `rightmostChain`.
  Decision: resolved — owner is this plugin (`pkg/geoblock/ip.go` `GetRemoteIPs`). Host Traefik sanitizes header bytes at the entrypoint and does not pass a selected hop into Yaegi `ServeHTTP` (`knowledge/research/ext_traefik_forwardedheaders_trustedips/`, `traefik@faa1eb59` `middlewareyaegi.go`). `ipStrategy.excludedIPs` is a peer reconstruction inside Traefik’s own middlewares and is not written onto `req`. Reuse `GetRemoteIPs` / `collectHops` / `rightmostChain`; do not copy Traefik’s walker; do not add a second reconstructor. Archive already named this owner (`openspec/changes/archive/2026-09-14-bin-rwmutex-published-handle/design.md`).
  By: explore

- Q: What does Traefik v3.5+ / v3.7.11 do with forwarded headers when the TCP peer is not in entrypoint `forwardedHeaders.trustedIPs`?
  Rank: additive asked — R6 names verify this before README; documenting it does not reshape existing callers.
  Decision: resolved — `insecure=false` and peer not in `trustedIPs`: Traefik **deletes** managed `X-Forwarded-*` / `X-Real-Ip`, then fills empties. At plugin time XFF is empty (`rewrite` does not append `RemoteAddr`); `X-Real-Ip` is the TCP peer; `req.RemoteAddr` is still the TCP peer. RFC 7239 `Forwarded` is not in the managed set. Source: `knowledge/research/ext_traefik_forwardedheaders_trustedips/` (`traefik@faa1eb59` `forwarded_header.go`). README may state the XFF-empty fact; keep `remoteAddress` last so there is still a hop.
  By: explore

- Q: Does the maintainer want an OpenSpec spec for IP header strategies?
  Rank: additive incidental — new catalog folder this change would create; Unknowns on requirement.md, no In-scope/acceptance line names a spec.
  Decision: assumed — yes: propose `core_geoblock_plugin_ip-header-strategy` with R1–R4 scenarios. FindSpecHost: new, high; candidates `core_geoblock_plugin_request-mode`, `std_go_iplookup_family-isolated`, `core_geoblock_observability_decision-header`, illegal 3-part `core_geoblock_ip-header-strategy`. Family `core_geoblock_plugin` already exists; this is a large new capability (new strategy, `trustedProxies`, alternative-header rule), not a 1–3 requirement fold. Neighbors keep empty-hop/`banIfError` and `Contains`; no second delta. Change kebab is not the 4th part.
  By: propose

- Q: Which real proxies emit `X-Forwarded-For` as separate HTTP field lines?
  Rank: additive incidental — README wording only; Unknowns say the R2 fix does not depend on the answer.
  Decision: assumed — describe the HTTP field-line case; do not name a product without a sourced research folder.
  By: explore

- Q: Should `TestNew_UnreclaimedHashDisposesAfterGrace` (PR #95 body: flaky 80 ms sleep on master) ride this PR?
  Rank: additive incidental — R7 says track separately; not an acceptance case for RightmostUntrusted.
  Decision: assumed — leave it. Not this change.
  By: explore
