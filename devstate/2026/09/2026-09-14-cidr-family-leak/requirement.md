# Requirement
IssueKey: 2026-09-14-cidr-family-leak

## Problem
IPv4 and IPv6 CIDR rules share one radix path. An IPv4 `a.b.c.d/32` also matches IPv6 whose first four bytes are `a b c d`; an IPv6 `/32` matches the IPv4 made of those bytes. An operator who allow-lists an IPv6 office prefix silently allow-lists unrelated IPv4 (`pass:allowed_ip_block` on a country-blocked client). The mirror case blocks innocent traffic.

## Current (code)
- `pkg/iplookup/iplookup.go` `insert`: IPv4 uses `bitStart=96` on the mapped 16-byte form; IPv6 uses `bitStart=0`; both walks start at `tree.root`. `IpLookupHelper` holds one `ipRadixTree`.
- `pkg/iplookup/iplookup.go` `contains`: same `bitStart` / `tree.root` pairing. `radixNode` has no family tag, so a match cannot be attributed to the family that inserted it.
- `pkg/geoblock/plugin.go` `decide` / `isAllowedIPBlocks` / `isBlockedIPBlocks`: consume `IpLookupFileMonitor.IsContained` (`pkg/iplookup/file_monitor.go`) with no family check.
- `pkg/iplookup/iplookup_test.go` `TestIpLookupHelper_MixedIPv4AndIPv6`: mixed-family coverage uses non-colliding prefixes (`192.168.1.0/24` vs `2001:db8::/32`).
- `contains` already returns `(found, prefixLength)`. A `/0` insert marks the root endpoint (`prefixLen=0`). `pkg/iplookup/iplookup_test.go` `TestIpLookupHelper_EdgeCases` and `TestIpLookupHelper_PrefixLengthAccuracy` pin catch-all match and longest-prefix within a family when both `0.0.0.0/0` and `::/0` are present.
- `pkg/geoblock/plugin.go` `decide`: specificity gate is `allowedNetworkLength > 0` (F-5 owner; not this ticket).
- `pkg/dbwrappers` MMDB/BIN pass the lookup IP to the vendor DB; they do not own CIDR radix matching. CIDR rules belong to `pkg/iplookup`.

## Desired
A CIDR rule matches only the address family it was written for. Longest-prefix behaviour stays within a family. Keep `IpLookupHelper` / `IpLookupFileMonitor` as the owner (`AddCIDR` / `IsContained`). Prefer two internal trees or a family discriminator on the endpoint — smallest durable delta, shaped to that helper. Product tests (not `zzz_proof_*` names) cover colliding prefixes: `1.2.3.4/32` vs `102:304::1`; `808:808::/32` vs `8.8.8.8`; request-path IPv6 allow vs blocked-country IPv4 (existing `newRoute` / `holdCtx` in `pkg/geoblock`).

## Affected
- `pkg/iplookup/iplookup.go` (`insert`, `contains`, `IpLookupHelper`)
- `pkg/iplookup/iplookup_test.go` (mixed-family coverage)
- `pkg/geoblock` request-path tests for the leak
- `pkg/geoblock/plugin.go` `decide` only if a found/length change is required to keep current `/0` tests passing
- README `allowedIPBlocks` / `blockedIPBlocks` (family isolation is not stated today)

## Out of scope
- F-1 BIN race
- F-3 empty `bypassHeaders` value
- F-4 updater `Stop`
- F-5 `/0` vs longest-prefix product change in `decide` unless explore shows it is the same owner and inseparable from family isolation
- Copying `zzz_proof_*` filenames
- F-6 through F-9 from the same bug hunt

## Unknowns
- Two trees vs family discriminator: both honour the ask; explore picks against the owner that already exists (`IpLookupHelper` / `ipRadixTree`).
- Whether a found/length split in `contains` is required to keep current `/0` tests passing after family isolation. `contains` already returns `found` separately; EdgeCases inserts both family catch-alls.

## Tensions
None between the ticket and dest code on the leak. F-5 is adjacent (same tree, different product ask) and stays out of scope unless explore finds it inseparable.
