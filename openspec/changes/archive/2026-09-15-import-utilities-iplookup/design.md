## Context

See proposal.md — Why. DestBranch `pkg/iplookup` owns both the radix and directory `.txt` load. Utilities v1.0.2 (`5bfd322`) publishes `Helper` / `New` / `AddCIDR(cidr, metadata)` / `Contains` on two trees and no file monitor. This repo already vendors utilities v1.0.1 for reclaim. Hop IP stays on Plugin (`explore.md`).

## Goals / Non-Goals

**Goals:**
- Require utilities v1.0.2, vendor, import `…/iplookup`.
- Delete `pkg/iplookup`. Load static + directory CIDRs in `pkg/geoblock` onto `*iplookup.Helper`.
- Call sites use `Contains` (discard metadata) and `AddCIDR(cidr, "")`.
- Port directory-list tests into `pkg/geoblock`.

**Non-Goals:**
- Leftover `pkg/iplookup` adapter.
- Importing unused utilities packages.
- Changing `decide` `/0` vs longest-prefix.
- Reopening PR 86.
- Copying utilities `*_test.go` or caller `zzz_proof_*`.

## Decisions

1. **Vendor, do not fork** — Same as reclaim: Yaegi loads third-party packages from `vendor/`. Copying `helper.go` into `pkg/` is a first-party fork. Alternative: leftover adapter package — rejected; the ticket prefers delete and `AllowedIPBlocksDir` already lives on geoblock Config.

2. **Geoblock owns list load** — `fileutils` is exists/copy/search, not CIDR parsing. A one-shot load at `NewCore` (static first, then walk `.txt`, missing dir debug) stays next to the Config keys. Alternative: new `pkg/` helper with a different name — extra package for one owner.

3. **Empty metadata** — Remote `AddCIDR` requires a metadata string. This change does not label prefixes. Pass `""`. Alternative: store the CIDR string as metadata — unused by `decide`.

4. **FindSpecHost** — Candidates: `std_go_reclaim_context-lease` (sibling import pattern, different package), `core_geoblock_plugin_request-mode` (when CIDR lists apply, not how families isolate). Verdict: **new** `std_go_iplookup_family-isolated` (high). Do not fold request-mode; that leaf already applies the lists.

## Risks / Trade-offs

- [Directory-list tests disappear with `pkg/iplookup`] → Mitigation: port missing-dir, static+directory, and invalid-line cases into `pkg/geoblock` before delete.
- [`go mod vendor` overwrites the oschwald Yaegi overlay] → Mitigation: re-run `scripts/apply-oschwald-yaegi-patch.ps1` after vendor, same as other bumps.
- [Utilities `Contains` errors on nil IP] → Mitigation: `decide` already fails `net.ParseIP` before the list checks.

## Migration Plan

Ship in the next plugin build. No YAML key change. Operators keep `allowedIPBlocks` / `blockedIPBlocks` and the directory keys. Rollback is revert the bump and restore `pkg/iplookup`.

## Open Questions

Ticket questions live on `devstate/explore.md`.
