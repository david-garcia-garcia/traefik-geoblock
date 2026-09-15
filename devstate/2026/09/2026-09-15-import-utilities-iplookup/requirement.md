# Requirement
IssueKey: 2026-09-15-import-utilities-iplookup

## Problem
This plugin stores CIDR allow/block lists in a first-party radix in `pkg/iplookup` on one shared tree. A prefix can match the other family. The ticket asks to consume `github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup` at v1.0.2 (family-isolated trees) and drop the local package unless explore finds a contract mismatch that needs a thin adapter. Do not land or continue PR 86 / `2026-09-14-cidr-family-leak`.

## Current (code)
- `go.mod` requires `github.com/david-garcia-garcia/traefik-middleware-utilities` **v1.0.1**. `vendor/github.com/david-garcia-garcia/traefik-middleware-utilities/` has `reclaim/table.go` only. No vendored `iplookup`.
- `pkg/iplookup/iplookup.go`: `IpLookupHelper` on one `ipRadixTree`. `AddCIDR(cidr string)`, `IsContained(net.IP) (bool, int, error)`, `NewEmptyIpLookupHelper` / `NewIpLookupHelper`. IPv4 walks start at bit 96 on that same tree as IPv6.
- `pkg/iplookup/file_monitor.go`: `IpLookupFileMonitor` / `NewIpLookupFileMonitor(cidrBlocks, directoryPath, logger)` loads static CIDRs then `.txt` files under the directory (missing dir is debug, not fatal). `IsContained` delegates to the helper.
- `pkg/geoblock/plugin.go`: `allowedIPBlocks` / `blockedIPBlocks` are `*iplookup.IpLookupFileMonitor` from `cfg.AllowedIPBlocks` + `AllowedIPBlocksDir` (and the blocked pair). `isAllowedIPBlocks` / `isBlockedIPBlocks` return `IsContained`. `decide` compares prefix lengths; the longest-prefix branch runs only when both lengths are `> 0` (a `/0` match is prefix 0).
- `pkg/geoblock/plugin_observe_test.go`: constructs monitors via `iplookup.NewIpLookupFileMonitor`.
- `knowledge/devdocs/core_geoblock_plugin_packages.md`: lists `iplookup` under `pkg/` helpers. Live `openspec/specs/` has no `iplookup` folder. CIDR allow/block rules live in `openspec/specs/core_geoblock_plugin_request-mode/spec.md`.
- Utilities v1.0.2 `iplookup` (research `knowledge/research/ext_traefik-middleware-utilities_iplookup/notes.md`, commit `5bfd322`): `Helper`, `New()`, `AddCIDR(cidr, metadata string)`, `Contains` → `(found, prefixLen, metadata, err)`, `RemoveCIDR`, `Reset`. Two trees. No file monitor.

## Desired
- Bump utilities from v1.0.1 to **v1.0.2**.
- Switch product call sites from `github.com/david-garcia-garcia/traefik-geoblock/pkg/iplookup` to `github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup`.
- Remove local `pkg/iplookup` unless explore finds a contract mismatch that requires a thin adapter; prefer delete + call-site update.
- Preserve geoblock CIDR allow/block behavior (`allowedIPBlocks` / `blockedIPBlocks` and directory lists). Family isolation comes from the utilities helper.
- Vendor as this plugin already vendors (Yaegi). Re-apply `scripts/apply-oschwald-yaegi-patch.ps1` after `go mod vendor` (same as other utility bumps).
- Update OpenSpec / `knowledge/devdocs` that name the local helper.

## Affected
- `go.mod`, `go.sum`, `vendor/github.com/david-garcia-garcia/traefik-middleware-utilities/`
- `pkg/iplookup/` (delete or leftover adapter)
- `pkg/geoblock/plugin.go`, `pkg/geoblock/plugin_observe_test.go`
- `knowledge/devdocs/core_geoblock_plugin_packages.md`
- OpenSpec only if a live spec names the local helper (none found besides CIDR behavior in `core_geoblock_plugin_request-mode`)

## Out of scope
- Reopening or merging PR 86 / branch `2026-09-14-cidr-family-leak`.
- Importing other unused utilities packages.
- Changing `decide` `/0` sentinel vs longest-prefix policy.
- CI `-race` overhaul.
- Copying or committing `zzz_proof_*` files from the caller workspace.

## Unknowns
- Utilities v1.0.2 has no `IpLookupFileMonitor`. Call sites and directory lists depend on it. Explore must choose: thin adapter that wraps `Helper`, move directory load into `pkg/geoblock`, or another existing owner. Ticket prefers delete unless mismatch requires an adapter.
- Public names differ (`Helper` / `Contains` / `AddCIDR` metadata). Whether call sites adapt in place or keep a local wrapper is the same explore choice.
- No live OpenSpec leaf names `pkg/iplookup`. What to update besides `knowledge/devdocs/core_geoblock_plugin_packages.md` is empty unless explore finds another host.

## Tensions
- Ticket prefers delete + call-site update. Current call sites use `IpLookupFileMonitor`, which v1.0.2 does not publish. That is a contract mismatch for explore, not a new product ask.
- Ticket forbids landing PR 86 / rewriting the local helper. Family isolation ships only by consuming utilities v1.0.2.
