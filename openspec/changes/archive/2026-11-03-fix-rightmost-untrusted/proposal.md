## Why

PR #95's `RightmostUntrusted` walks hops from the right and skips `trustedProxies`, but it runs on `GetRemoteIPs`, which drops repeated IPs and reads only the first header line, and it concatenates every configured `ipHeaders` entry. A client can then choose the country the README says they cannot. Dest has no `RightmostUntrusted` yet; this change adds the strategy so that guarantee holds.

## What Changes

- Add `ipHeaderStrategy: RightmostUntrusted` and `trustedProxies`. Implement from dest; do not patch the contributor branch.
- On that path only: evaluate the hop list as received (no dedup), read every field line of the selected header, treat forwarding `ipHeaders` as alternatives (first present wins), and close the chain with `remoteAddress` when listed.
- `trustedProxies` uses the shared `iplookup.Helper` (family isolation). A non-empty list with any other strategy fails creation.
- Trace / error logs on that path keep the full pre-filter chain and the selected hop.
- README documents the alternative-header rule, `remoteAddress` last, Traefik untrusted-peer XFF-empty fact, and the residual operator duty to strip unused configured headers.
- Existing `CheckAll` / `CheckFirst` / `CheckFirstNonePrivate` collection stays first-line `Get` + first-occurrence dedup (out of scope to change).

## Capabilities

### New Capabilities

- `core_geoblock_plugin_ip-header-strategy`: hop collection for `RightmostUntrusted` (`collectHops` / `rightmostChain`), first-present forwarding header, `remoteAddress` last, `trustedProxies` membership and load-time rules. FindSpecHost: new, high.

### Modified Capabilities

None. Empty hop / `banIfError` stays `core_geoblock_plugin_request-mode`. `Contains` family isolation stays `std_go_iplookup_family-isolated`. Trace attributes stay in design/tasks (not a decision-header delta). CheckAll/CheckFirst collection freeze is What-is-out, not a SHALL.

## Impact

- `pkg/geoblock/ip.go` — split `GetRemoteIPs`; `rightmostChain`; `rightmostUntrusted`; trusted-proxy helper builder
- `pkg/geoblock/plugin.go` — `Plugin` helper field; `NewCore`; `ServeHTTP` branch; `logLookupError` call site
- `pkg/geoblock/config.go` — `TrustedProxies`; `Prepare` (strategy enum, `remoteAddress` last, `trustedProxies` only with this strategy)
- `pkg/geoblock/zzz_plugin_rightmost_untrusted_test.go` — T1–T13
- `pkg/geoblock/zzz_plugin_ipheaders_test.go` — existing dedup tests must pass unchanged
- `README.md` — Client IP section
- **Operators:** new opt-in strategy. Default `ipHeaders` stays valid. `trustedProxies` is a new field; pairing it with CheckAll/CheckFirst/CheckFirstNonePrivate fails creation.
