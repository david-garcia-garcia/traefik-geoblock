## Why

`IpLookupHelper` stores IPv4 and IPv6 CIDRs on one radix root. An IPv4 `a.b.c.d/32` also matches IPv6 whose first four bytes are `a b c d`, and an IPv6 `/32` matches the IPv4 made of those bytes. An operator who allow-lists an IPv6 office prefix silently allow-lists unrelated IPv4 (`pass:allowed_ip_block` on a country-blocked client); the mirror case blocks innocent traffic.

## What Changes

- **BREAKING (behavior):** a CIDR in `allowedIPBlocks` / `blockedIPBlocks` (and the matching directory lists) matches only the address family it was written for. Cross-family hits such as `1.2.3.4/32` vs `102:304::1` and `808:808::/32` vs `8.8.8.8` stop matching. Longest-prefix stays inside one family.
- Keep `IpLookupHelper` / `IpLookupFileMonitor` as the owner. Public `AddCIDR` / `IsContained` / `Count` stay. `decide` does not classify family and does not change its `> 0` specificity gate.
- Product tests (not `zzz_proof_*`) cover colliding prefixes in `pkg/iplookup` and a request-path IPv6 allow vs blocked-country IPv4 in `pkg/geoblock`.
- README states that `allowedIPBlocks` / `blockedIPBlocks` match only the CIDR’s family.

## Capabilities

### New Capabilities

- `core_geoblock_iplookup_family-match`: a written CIDR matches a lookup IP only when both are the same address family; longest-prefix and catch-all `/0` stay family-local.

### Modified Capabilities

None.

## Impact

- `pkg/iplookup/iplookup.go` (`IpLookupHelper`, `insert`, `contains`)
- `pkg/iplookup/iplookup_test.go` (colliding mixed-family cases)
- `pkg/geoblock/plugin_policy_test.go` (request-path leak via `newRoute` / `holdCtx` / `testRequest`)
- `README.md` (`allowedIPBlocks` / `blockedIPBlocks`)
- `knowledge/devdocs/core_geoblock_iplookup.md` (already describes family trees; confirm after apply)
- **Operators:** an IPv6 CIDR no longer matches colliding IPv4 (and the reverse). List both families when both should match. No new config key.
- **Non-goal:** F-5 (`decide` treating `/0` as a sentinel vs longest-prefix). Same-family `/0` still returns `(true, 0)`.
