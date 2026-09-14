## Context

See proposal.md — Why. `IpLookupHelper` holds one `ipRadixTree`. `insert` and `contains` both start at `tree.root`. IPv4 walks from bit 96 of the 16-byte mapped form; IPv6 walks from bit 0. `radixNode` has no family tag, so the first 32 levels are a shared path. `contains` already returns `(found, prefixLength)`. Public callers of `AddCIDR` / `IsContained` are `pkg/iplookup/file_monitor.go` and `pkg/geoblock/plugin.go` (`isAllowedIPBlocks` / `isBlockedIPBlocks`). Identity of the hop stays on `GetRemoteIPs` then `net.ParseIP` in `decide`; family of a match stays on the helper.

## Goals / Non-Goals

**Goals:**

- Family isolation inside `IpLookupHelper` so a CIDR cannot match the other family.
- Keep `AddCIDR` / `IsContained` / `Count` as the public surface.
- Keep `ipRadixTree` one-family (no endpoint tag).
- Product tests for the measured collisions, including `/0` cross-family negatives and one request-path case.

**Non-Goals:**

- Changing `decide`’s `allowedNetworkLength > 0` gate (F-5).
- Changing the `IsContained` return shape.
- Classifying family in `decide` or from hop headers / MMDB / BIN.
- A third family for IPv4-mapped addresses.

## Decisions

- **Two internal trees on `IpLookupHelper`.** `AddCIDR` and `IsContained` already branch on `ip.To4() != nil`. Give the helper an IPv4 tree and an IPv6 tree; pick the tree with that same branch. Alternative: one tree plus a family discriminator on `radixNode` — rejected. `0.0.0.0/0` and `::/0` both mark the shared root (`prefixLen=0`) and would overwrite or need two endpoint flags. Two trees isolate that node without tagging `radixNode`. Explore Decision: two internal trees.

- **Leave `ipRadixTree.insert` / `contains` bitStart logic in place.** Each tree only ever holds one family, so the existing IPv4 mapped-form walk (`bitStart=96`) and IPv6 walk from bit 0 stay valid on that tree’s root. Alternative: rewrite IPv4 as a 32-bit walk from bit 0 — rejected; larger delta, same observable match.

- **No `IsContained` API split and no `decide` edit.** Two family trees return `(true, 0)` for a same-family `/0` and `false` for the other family. `TestIpLookupHelper_EdgeCases` and `TestIpLookupHelper_PrefixLengthAccuracy` stay valid. Explore Decision: no split.

- **Classify with existing `To4() != nil`.** IPv4-mapped IPv6 follows the IPv4 tree. Do not add a plugin-level mapped check. Explore Decision: keep `To4()`.

- **`Count` remains the insert count** across both trees. Empty family tree → that family’s lookup is not contained.

## Risks / Trade-offs

- [Operators who listed only an IPv6 prefix and relied on colliding IPv4 matching] → intended break; list both families. README states family isolation.
- [F-5 still lets a same-family `/0` allow skip longest-prefix vs a tighter block] → out of scope; `knowledge/debt/2026-09-14-decide-slash-zero-sentinel.md`.
- [IPv4-mapped CIDRs (`::ffff:a.b.c.d/128`) land on the IPv4 tree] → same as today’s `To4()` branch; document in the usage packet gotcha that already exists.

## Migration Plan

Ship. No config key. Operators who need a range in both families list both CIDRs. Rollback is a revert.
