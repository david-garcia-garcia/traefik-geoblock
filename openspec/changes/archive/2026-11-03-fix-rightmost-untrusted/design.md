## Context

See proposal.md — Why. Dest `GetRemoteIPs` (`pkg/geoblock/ip.go`) reads each `ipHeaders` entry with `Header.Get` (or `req.RemoteAddr`), splits on `,`, `cleanIPAddress`, and dedupes keeping the first occurrence. `ServeHTTP` always calls it (~L319). `Prepare` accepts only `CheckAll` / `CheckFirst` / `CheckFirstNonePrivate`. `skipIP` is those two CheckFirst modes. Empty hop / `banIfError` is `core_geoblock_plugin_request-mode`. CIDR membership is `iplookup.Helper` (`std_go_iplookup_family-isolated`); `loadIPBlockHelper` also walks directory `.txt` lists. Traefik does not pass a selected hop into Yaegi `ServeHTTP` (`knowledge/research/ext_traefik_forwardedheaders_trustedips/`). Neighbor leftover change `policy-guards-and-seed-fallback` is not this ticket.

## Goals / Non-Goals

**Goals:**

- Split hop collection from first-occurrence dedup so `RightmostUntrusted` can walk the raw chain.
- `ServeHTTP` branch: `rightmostChain` then `rightmostUntrusted`; `GetRemoteIPs` stays the other strategies.
- `trustedProxies` on a dedicated `iplookup.Helper` builder (plain address → `/32` or `/128`).
- Trace `ipChain` from the pre-filter chain plus the selected hop (R5).
- README R6 sourced from the Traefik research folder.

**Non-Goals:**

- `Header.Values` or drop-dedup on `CheckAll` / `CheckFirst` / `CheckFirstNonePrivate`.
- RFC 7239 `Forwarded`.
- Per-header trusted sources.
- A third `skipIP` branch.
- Reusing `loadIPBlockHelper` or Traefik `ipStrategy.excludedIPs`.
- Treating an unparsable rightmost hop as “no client IP”.
- Spec deltas on `request-mode` or `iplookup` (those owners stay).

## Decisions

- **Owner split, one reconstruction.** `collectHops(req, names, allLines)` reads the named headers, splits, `cleanIPAddress`, no dedup. `allLines` true → `Header.Values` in order; false → `Header.Get`. `GetRemoteIPs` = `dedupeFirst(collectHops(req, p.ipHeaders, false))`. `rightmostChain` loops forwarding names with `collectHops(..., true)` and stops at the first non-empty result, then appends `cleanIPAddress(req.RemoteAddr)` when `remoteAddress` is listed. Alternative: dedupe last occurrence — rejected (R1). Alternative: concatenate every present header — rejected (R3). Alternative: copy Traefik's walker — rejected; host does not pass that result.
- **`ServeHTTP` switches on strategy once.** `RightmostUntrusted` sets `hops := rightmostChain(req)`, `remoteIPs := rightmostUntrusted(hops)`, `ipChain := traceIPChain(hops)`. Other strategies keep `GetRemoteIPs`. `skipIP` unchanged: a one-hop slice needs no extra branch. Alternative: filter after `GetRemoteIPs` (PR #95) — rejected; that is the dedup bypass.
- **Trusted-proxy helper, not `[]*net.IPNet`.** New builder on `Plugin` (`*iplookup.Helper`). Parse once in `NewCore`. `Prepare` may call the same builder and discard the result for early syntax; no second parser. `rightmostUntrusted` `net.ParseIP` once per hop then `helper.Contains`. `Contains` error → not trusted. Do not call `loadIPBlockHelper` (directory job). Alternative: keep PR `inNets` — rejected; family isolation would drift.
- **`Prepare` cheap gates.** Accept `RightmostUntrusted`. `remoteAddress` present but not last → error naming the list. Non-empty `trustedProxies` with another strategy → error. Missing `remoteAddress` → `NewCore` WARN once, not a reject. Other strategies: no new validation.
- **Unparsable rightmost hop stays the evaluated hop** (`XX` / `defaultAllow`). Requirement Tensions: it adds no new bypass. Alternative: empty-hop / `banIfError` — rejected.
- **R5 attributes on the existing logger.** `ipChain` from pre-filter `hops`. Selected hop on the existing `ip` attribute (or `selected_ip` if `ip` is already the lookup target). All-trusted: DEBUG, chain included. Pass the full chain into `logLookupError` so the empty-`ipChain` fallback does not print a single hop.
- **Tests.** New `pkg/geoblock/zzz_plugin_rightmost_untrusted_test.go` (T1–T12). Existing `zzz_plugin_ipheaders_test.go` dedup cases must pass unchanged (T13). Optional compose/Pester case if a PathPrefix can carry the new strategy without rewriting CheckAll routes.
- **README.** Alternative-header rule, `remoteAddress` last, residual strip duty, Traefik untrusted-peer XFF-empty / `X-Real-Ip` filled (sourced). Do not name a proxy product for multi-line XFF. Keep “same rule as Traefik `ipStrategy.excludedIPs`” only after R1–R3.

## Risks / Trade-offs

- [Client forges a configured header a trusted proxy does not strip] → first-present wins, so the client picks the header. Residual risk; per-header trusted sources are out of scope. Document the operator duty. Listing `remoteAddress` last closes direct-to-Traefik.
- [Compose sets `forwardedHeaders.insecure=true`] → it does not demonstrate Traefik's untrusted delete. README states the sourced untrusted path; do not claim compose shows it.
- [CheckAll still blind to extra field lines] → same as dest; out of scope. A later change can flip `allLines` for those strategies.
- [Empty `trustedProxies` with `remoteAddress` last] → the direct peer is always the evaluated hop, headers ignored. Correct for “no trusted proxy”; operators who meant “trust everyone who can reach Traefik” must list those peers.

## Migration Plan

Ship. `RightmostUntrusted` is opt-in. Default `ipHeaders` stay valid with it. Rollback is revert. No dest operator is on this strategy yet.

## Open Questions

None. Remaining explore rows stay assumed (`core_geoblock_plugin_ip-header-strategy` id, no product named for multi-line XFF, unreclaimed-hash flake is not this change).
