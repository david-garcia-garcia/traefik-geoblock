# CIDR lookup helper

## Language

**CIDR family**:
The IPv4 or IPv6 membership of a written CIDR or of a lookup IP. A match is valid only when both belong to the same family.
_Avoid_: mixed radix path; treating the mapped-form bit offset as the family test

## Overview

`IpLookupHelper` stores `allowedIPBlocks` / `blockedIPBlocks` and answers whether an IP is in those CIDRs. `IpLookupFileMonitor` loads static lists plus `.txt` files, then delegates `IsContained`. Family isolation lives in the helper, not in `decide`.

## How to use

- Insert with `AddCIDR`. Look up with `IsContained`. Keep those two as the public owner.
- Put IPv4 CIDRs on the IPv4 tree and IPv6 CIDRs on the IPv6 tree. Classify with `ip.To4() != nil` (IPv4-mapped IPv6 follows IPv4).
- Look up only the tree that matches the lookup IP’s family. Longest-prefix stays inside that tree.
- Do not share one radix root across families. A `/0` insert marks that family’s root; a shared root would match the other family with `prefixLen=0`.
- `decide` consumes `(found, prefixLength)` and does not re-classify family. Do not add a family check at the plugin call site.

## Pattern snippet

```go
helper := NewEmptyIpLookupHelper()
if err := helper.AddCIDR(cidr); err != nil {
	return err
}
found, prefixLen, err := helper.IsContained(ipAddr)
```

## Key files

- `pkg/iplookup/iplookup.go` — `IpLookupHelper`, `AddCIDR`, `IsContained`, family trees
- `pkg/iplookup/file_monitor.go` — directory load, then `IsContained`
- `pkg/geoblock/plugin.go` — `isAllowedIPBlocks` / `isBlockedIPBlocks` consume the monitor
- `pkg/iplookup/iplookup_test.go` — mixed-family and longest-prefix cases
- `pkg/geoblock/plugin_policy_test.go` — request-path CIDR allow/deny (`newRoute` / `holdCtx`)

## Gotchas

- `net.IP.To4() != nil` is true for IPv4-mapped IPv6 (`::ffff:a.b.c.d`). Those CIDRs and lookups follow the IPv4 tree.
- Catch-all `0.0.0.0/0` and `::/0` are family-local. They must not sit on one shared endpoint.
- Longest-prefix comparison between allow and block lists is `decide` (`allowedNetworkLength > 0`). That gate is not this helper’s job.
