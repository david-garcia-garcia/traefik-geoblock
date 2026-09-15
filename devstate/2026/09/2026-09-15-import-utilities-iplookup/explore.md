# Explore
IssueKey: 2026-09-15-import-utilities-iplookup
Verdict: in progress

## Concepts

DestBranch stores CIDR allow/block lists in a first-party radix (`pkg/iplookup`) on **one** tree. IPv4 walks start at bit 96 on that same tree as IPv6, so a prefix can match the other family. The ticket is to consume `github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup` at **v1.0.2** (family-isolated trees) and drop the local package. Do not land PR 86 / `2026-09-14-cidr-family-leak`. Family isolation ships only by consuming the published helper.

```
DestBranch today                         v1.0.2 (target)
----------------                         ---------------
pkg/iplookup.IpLookupHelper              utilities iplookup.Helper
one ipRadixTree                          ipv4Tree + ipv6Tree
AddCIDR(cidr)                            AddCIDR(cidr, metadata string)
IsContained -> (bool, int, error)        Contains -> (found, prefixLen, metadata, err)
IpLookupFileMonitor + dir .txt load      no file monitor (product owns lists)
go.mod utilities v1.0.1 (reclaim only)   bump to v1.0.2; vendor iplookup too
```

**Family-isolated helper.** Published `…/iplookup` at tag v1.0.2, commit `5bfd322`. `treeFor` picks the tree from `ip.To4() != nil`. A prefix cannot match the other family. Stdlib only (`errors`, `fmt`, `net`, `sync`). Remote: `knowledge/research/ext_traefik-middleware-utilities_iplookup/notes.md`.

**Directory lists.** `AllowedIPBlocks` / `BlockedIPBlocks` plus `AllowedIPBlocksDir` / `BlockedIPBlocksDir` (`.txt`, one CIDR per line, `#` comments). Missing dir is debug, not fatal. That load lives in `pkg/iplookup/file_monitor.go` today. Utilities v1.0.2 does not publish it. `pkg/fileutils` is exists/copy/search, not CIDR parsing.

**Plugin fields.** `pkg/geoblock/plugin.go` holds two `*iplookup.IpLookupFileMonitor`. `NewCore` builds them. `isAllowedIPBlocks` / `isBlockedIPBlocks` call `IsContained`. `decide` compares prefix lengths; the longest-prefix branch runs only when both lengths are `> 0` (a `/0` match is prefix 0). Out of scope to change that sentinel.

**Call sites** (searched `*.go` excluding `vendor/` for `pkg/iplookup`, `IpLookupFileMonitor`, `NewIpLookupFileMonitor`, `IsContained`): `pkg/geoblock/plugin.go` (2 `NewIpLookupFileMonitor`, 2 fields, 2 `IsContained`); `pkg/geoblock/plugin_observe_test.go` (2 empty-monitor constructions). Local package: 4 files under `pkg/iplookup/`. Docs: `knowledge/devdocs/core_geoblock_plugin_packages.md` lists `iplookup` under `pkg/`. Live `openspec/specs/` has no `iplookup` folder. CIDR allow/block rules live in `openspec/specs/core_geoblock_plugin_request-mode/spec.md`. Archived change `2026-08-27-split-packages-drop-legacy-config` named the local helper; do not rewrite archive.

**Identity.** Hop IP stays on `IPHeaders` / `ipHeaderStrategy` / `decide`. This change does not reconstruct client address, user, tenant, Host, or trust hop.

**Vendor.** Same as reclaim: `go.mod` require + `go mod vendor` + re-apply `scripts/apply-oschwald-yaegi-patch.ps1`. Do not copy utilities tests (`go mod vendor` omits `*_test.go`). Parallel: `openspec/specs/std_go_reclaim_context-lease` (SHALL import vendored reclaim; SHALL NOT keep `pkg/reclaim`).

## Decisions

- Bump `github.com/david-garcia-garcia/traefik-middleware-utilities` from v1.0.1 to **v1.0.2**, vendor, import `…/iplookup`. Delete first-party `pkg/iplookup` (including local helper tests). Do not fork the helper into `pkg/`. Do not land PR 86.
- Contract mismatch (no remote file monitor) does **not** keep a leftover `pkg/iplookup` adapter. Directory + static CIDR load moves into `pkg/geoblock` next to `AllowedIPBlocksDir` / `BlockedIPBlocksDir`. Plugin fields become `*iplookup.Helper`. Call sites use `New()`, `AddCIDR(cidr, "")`, `Contains` (discard metadata).
- Preserve `decide` `/0` vs longest-prefix. Do not import other unused utilities packages.
- Port product directory-load cases into `pkg/geoblock` tests (DestBranch only covers them in `pkg/iplookup/file_monitor_test.go`). Family-isolation unit tests stay upstream.
- Later phases update `knowledge/devdocs/core_geoblock_plugin_packages.md` (drop `iplookup` from the `pkg/` list; point at vendored utilities like reclaim). Propose runs FindSpecHost; no live spec leaf names the local helper.

## Open questions

- Q: Utilities v1.0.2 has no `IpLookupFileMonitor`. Keep a thin leftover `pkg/iplookup` adapter, or delete the package and own directory load in `pkg/geoblock`?
  Rank: bounded asked — 2 product files import `pkg/iplookup` (`plugin.go`, `plugin_observe_test.go`) plus 4 files in `pkg/iplookup`; searched `*.go` excluding `vendor/` for `pkg/iplookup`, `IpLookupFileMonitor`, `NewIpLookupFileMonitor`; Desired “Remove the local pkg/iplookup package unless explore finds a contract mismatch that requires a thin adapter; prefer delete + call-site update”
  Decision: assumed — delete `pkg/iplookup`; move static + directory `.txt` load into `pkg/geoblock`; store `*iplookup.Helper`. The mismatch is real but the product already owns `AllowedIPBlocksDir`; a leftover first-party `iplookup` package is the thing the ticket prefers to remove.
  By: explore

- Q: Call sites use `IsContained` and `AddCIDR(cidr)`. Remote is `Contains` (extra metadata) and `AddCIDR(cidr, metadata)`. Adapt in place or wrap?
  Rank: bounded asked — same 2 product files; Desired “Switch all product call sites … to …/traefik-middleware-utilities/iplookup”
  Decision: assumed — adapt in place. `AddCIDR(cidr, "")`. `Contains` keeps `found`, `prefixLen`, `err`; discard metadata. No labels, no wrapper type.
  By: explore

- Q: Who already owns the hop IP this helper would classify?
  Rank: additive asked — criterion “When the work would set or reconstruct identity”; this change only answers CIDR membership
  Decision: assumed — reuse `Plugin` hop extraction (`IPHeaders` / `ipHeaderStrategy` / `decide`). Do not re-parse the request or invent a second client-address owner. Utilities `Contains` only answers membership for the `net.IP` already chosen.
  By: explore

- Q: No live OpenSpec leaf names `pkg/iplookup`. What spec host does propose write besides `knowledge/devdocs/core_geoblock_plugin_packages.md`?
  Rank: additive asked — Desired “Update OpenSpec / knowledge/devdocs that name the local helper”; live `openspec/specs/` has no `iplookup` folder
  Decision: assumed — propose runs FindSpecHost. Prefer a new leaf parallel to `std_go_reclaim_context-lease` (import vendored utilities `iplookup`; SHALL NOT keep `pkg/iplookup`). Fold a family-isolation SHALL into `core_geoblock_plugin_request-mode` only if FindSpecHost says that existing leaf already owns CIDR match rules. Do not rewrite archived change folders.
  By: explore

- Q: DestBranch directory-list tests live only in `pkg/iplookup/file_monitor_test.go`. After delete, where do they go?
  Rank: bounded asked — 1 test file owns directory load; Desired “Preserve geoblock CIDR allow/block behavior (allowedIPBlocks / blockedIPBlocks and directory lists)”
  Decision: assumed — port missing-dir, static+directory, and invalid-line cases into `pkg/geoblock` tests. Delete local helper / radix unit tests (upstream owns those). Do not copy `zzz_proof_*`.
  By: explore
