## 1. Family trees on the helper

- [x] 1.1 Split `IpLookupHelper` onto an IPv4 tree and an IPv6 tree. Route `AddCIDR` / `IsContained` with existing `ip.To4() != nil`. Leave `ipRadixTree` one-family. Keep `Count` as the insert count. Do not change the `IsContained` return shape.
- [x] 1.2 Do not edit `pkg/geoblock/plugin.go` `decide` (no family check, no `> 0` gate change).

## 2. Helper tests

- [x] 2.1 In `pkg/iplookup/iplookup_test.go`, assert colliding prefixes do not match: `1.2.3.4/32` vs `102:304::1`; `808:808::/32` vs `8.8.8.8`; same-family positives; `::ffff:1.2.3.4` follows IPv4. Product test names only (not `zzz_proof_*`).
- [x] 2.2 Assert `/0` is family-local: `0.0.0.0/0` does not contain IPv6; `::/0` does not contain IPv4; both catch-alls still return `(true, 0)` for their own family. Keep existing EdgeCases and PrefixLengthAccuracy passing.

## 3. Request path

- [x] 3.1 In `pkg/geoblock/plugin_policy_test.go` via `newRoute` / `holdCtx` / `testRequest`: `allowedIPBlocks` `808:808::/32` plus `blockedCountries` `US` must not allow `8.8.8.8`. Same CIDR with `defaultAllow` false must still allow `808:808::1`.

## 4. Docs

- [x] 4.1 README: `allowedIPBlocks` / `blockedIPBlocks` match only the family the CIDR was written for.
- [x] 4.2 Confirm `knowledge/devdocs/core_geoblock_iplookup.md` still matches two family trees after the helper change.
