# Explore

## Concepts

CIDR allow/deny is owned by `pkg/iplookup`. `IpLookupHelper` holds the radix and exposes `AddCIDR` / `IsContained`. `IpLookupFileMonitor` loads static lists and `.txt` files, then delegates. `pkg/geoblock/plugin.go` `isAllowedIPBlocks` / `isBlockedIPBlocks` call the monitor. `pkg/dbwrappers` MMDB/BIN do not match operator CIDRs.

`GetRemoteIPs` (`pkg/geoblock/ip.go`) owns hop address strings. `decide` parses them with `net.ParseIP` and passes that `net.IP` into `IsContained`. Family of a lookup is that IP’s `To4()` result. Family of a rule is `ParseCIDR` plus the same `To4()` test on insert. Neither helper re-reads hop headers.

Today `IpLookupHelper` holds one `ipRadixTree`. `insert` and `contains` both start at `tree.root`. IPv4 walks from bit 96 of the 16-byte mapped form; IPv6 walks from bit 0. `radixNode` has no family tag. The first 32 levels are therefore a shared path: an IPv4 `/32` is the same walk as an IPv6 whose first four bytes collide, and the reverse.

```
  insert / contains
          │
          ▼
     tree.root  ← shared
      /      \
   bit 0    bit 1     IPv6 starts here
      │
     … 96 bits …
      │
   IPv4 mapped bits   IPv4 starts here (bitStart=96)
```

Measured on dest (throwaway `go run` against this worktree’s `pkg/iplookup`, not product tests):

| CIDR | lookup IP | found | prefix | wanted |
| --- | --- | --- | --- | --- |
| `1.2.3.4/32` | `102:304::1` | true | 32 | false |
| `1.2.3.4/32` | `1.2.3.4` | true | 32 | true |
| `808:808::/32` | `8.8.8.8` | true | 32 | false |
| `808:808::/32` | `808:808::1` | true | 32 | true |
| `0.0.0.0/0` | `2001:db8::1` | true | 0 | false |
| `::/0` | `8.8.8.8` | true | 0 | false |

The `/0` rows are the same shared-root leak, not F-5. Inserting `0.0.0.0/0` marks the shared root (`prefixLen=0`), so every IPv6 lookup sees that endpoint before walking. Two family trees isolate that node. A family discriminator on one tree would need two endpoint flags on the root so `0.0.0.0/0` and `::/0` do not overwrite each other.

F-5 is a different product ask in `decide`:

```
if (allowedNetworkLength < blockedNetworkLength) && (allowedNetworkLength > 0) && (blockedNetworkLength > 0)
```

A same-family `/0` still returns `(true, 0)` after isolation. The `> 0` gate then lets a catch-all allow skip longest-prefix vs a tighter block. Owner: `plugin.go` `decide`. Family isolation owner: `IpLookupHelper`. Not the same job. Not inseparable. Leave F-5 out.

`contains` already returns `(found, prefixLength)`. `TestIpLookupHelper_EdgeCases` inserts both family catch-alls and only asserts `found`. `TestIpLookupHelper_PrefixLengthAccuracy` pins longest-prefix within a family, including `/0`. Two trees keep that shape: IPv4 lookup walks only the IPv4 tree, so `0.0.0.0/0` is `(true, 0)` and `::/0` is not visible. No found/length API split, and no `decide` edit, is required to keep those tests passing.

Public callers of `AddCIDR` / `IsContained` (production, searched `pkg/**` `*.go` for `AddCIDR`, `IsContained`, `NewIpLookupHelper`, `NewEmptyIpLookupHelper`):

- `pkg/iplookup/file_monitor.go` — `NewEmptyIpLookupHelper`, `AddCIDR` (static + directory), `IsContained` wrapper
- `pkg/geoblock/plugin.go` — `isAllowedIPBlocks`, `isBlockedIPBlocks` → monitor `IsContained` (2 call sites)

`ipRadixTree.insert` / `contains` have no callers outside `pkg/iplookup/iplookup.go`. Mixed-family coverage today uses non-colliding prefixes (`192.168.1.0/24` vs `2001:db8::/32`) in `TestIpLookupHelper_MixedIPv4AndIPv6`. Request-path CIDR cases live in `pkg/geoblock/plugin_policy_test.go` via `newRoute` / `holdCtx` / `testRequest`. No `zzz_proof_*` in this worktree.

README `allowedIPBlocks` / `blockedIPBlocks` says “more specific prefix wins” and does not say family isolation. Affected. Document that a rule matches only its CIDR family.

No active OpenSpec change (`openspec list --json` → `changes: []`). Usage gap: no packet for the CIDR helper; wrote `knowledge/devdocs/core_geoblock_iplookup.md`. No third-party CIDR owner; research indexes consumed, nothing to write.

## Decisions

- Keep `IpLookupHelper` / `IpLookupFileMonitor` as the owner. Do not classify family in `decide`.
- Implement family isolation as two internal trees on `IpLookupHelper` (IPv4 tree and IPv6 tree). `ipRadixTree` stays one-family. Public `AddCIDR` / `IsContained` / `Count` stay. Smallest durable delta: insert and contains already branch on `To4()`; `/0` becomes family-local without tagging `radixNode`.
- Do not change the `IsContained` return shape. Do not edit `decide`’s `> 0` gate in this ticket.
- Classify family with existing `ip.To4() != nil` (IPv4-mapped IPv6 follows IPv4).
- Product tests (not `zzz_proof_*`): colliding prefixes in `pkg/iplookup/iplookup_test.go` (`1.2.3.4/32` vs `102:304::1`, `808:808::/32` vs `8.8.8.8`, plus `/0` cross-family negatives); request-path IPv6 allow vs blocked-country IPv4 in `pkg/geoblock/plugin_policy_test.go` using `newRoute` / `holdCtx`.
- README: state that `allowedIPBlocks` / `blockedIPBlocks` match only the family the CIDR was written for.
- F-5 stays out of scope. Noted as `knowledge/debt/2026-09-14-decide-slash-zero-sentinel.md`.

## Open questions

- Q: Two internal trees or a family discriminator on the shared endpoint?
  Rank: additive asked — Desired names both options and asks explore to pick against `IpLookupHelper` / `ipRadixTree`; adding unexported tree fields on that helper; existing `AddCIDR` / `IsContained` callers keep working (3 production `IsContained` sites: `file_monitor.go`, `plugin.go` ×2; `insert`/`contains` only in `iplookup.go`)
  Decision: assumed — two internal trees. A discriminator must special-case the shared root so `0.0.0.0/0` and `::/0` do not overwrite one endpoint; two trees reuse the existing `To4()` branch and leave `ipRadixTree` family-agnostic.
  By: explore

- Q: Is a found/length split in `contains` required to keep current `/0` tests passing after family isolation?
  Rank: additive asked — Unknowns names this; `contains` already returns `(found, prefixLength)` and EdgeCases inserts both family catch-alls
  Decision: assumed — no split and no `decide` edit. Two family trees return `(true, 0)` for a same-family `/0` and `false` for the other family. EdgeCases and PrefixLengthAccuracy stay valid without an API change.
  By: explore

- Q: Who already owns the client address and the CIDR family of a match?
  Rank: additive asked — identity/family of the hop and of the rule; Desired keeps `IpLookupHelper` as match owner
  Decision: resolved — `GetRemoteIPs` owns hop strings (`pkg/geoblock/ip.go`). `decide` parses with `net.ParseIP` and passes that value in. `AddCIDR` owns the rule family from `ParseCIDR` + `To4()`. `IsContained` classifies the lookup IP the same way. Do not reconstruct family from hop headers or from MMDB/BIN.
  By: explore

- Q: Is F-5 (`decide` `/0` vs longest-prefix) the same owner and inseparable from family isolation?
  Rank: bounded incidental — Out of scope names F-5 unless same owner and inseparable; the `> 0` gate has 1 call site (`pkg/geoblock/plugin.go` `decide`)
  Decision: resolved — not inseparable. Family isolation is `IpLookupHelper` trees. F-5 is `decide` treating `prefixLen=0` as “no specific CIDR”. Measured `/0` cross-family hits are the shared-root leak, fixed by two trees without touching the gate. Do not take F-5.
  By: explore

- Q: How should IPv4-mapped IPv6 CIDRs and lookups (`::ffff:a.b.c.d`) be classified?
  Rank: additive incidental — no criterion names mapped-form CIDRs; keep the existing `To4()` branch in `insert`/`contains`
  Decision: assumed — keep `To4() != nil` as IPv4. Do not add a third family or a plugin-level mapped check.
  By: explore
