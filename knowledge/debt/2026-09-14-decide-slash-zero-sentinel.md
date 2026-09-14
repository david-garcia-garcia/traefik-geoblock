# `decide` `/0` sentinel vs longest-prefix

IssueKey: 2026-09-14-cidr-family-leak
Size: large
Action: note

## Why this follow-up
`pkg/geoblock/plugin.go` `decide` compares allow vs block CIDR specificity with `allowedNetworkLength > 0` and `blockedNetworkLength > 0`. A `/0` match returns `prefixLen=0`, so a catch-all allow skips the longest-prefix branch and wins over a more specific block (and the mirror). That is a product choice about `/0` vs longest-prefix, not family isolation.

## Why it was not taken
Requirement **Out of scope** names F-5 unless explore shows the same owner and inseparable from family isolation. Explore measured the opposite: family isolation belongs to `IpLookupHelper`; the `/0` gate belongs to `decide`. Two family trees still return `(true, 0)` for a same-family catch-all, so the gate can stay.

## Risks
Operators who put `0.0.0.0/0` in `allowedIPBlocks` and a tighter range in `blockedIPBlocks` still see the allow win after family isolation. README “more specific prefix wins” is then false for `/0`.
