## Context

See proposal.md — Why. `tools/dbdownload` GETs the official LITE ZIP and writes `seeds/IP2LOCATION-LITE-DB1.IPV6.BIN` only if `1.1.1.1` `country_short` is `US`. Official LITE (committed and current extract) returns `AU`. `go:generate` cannot write the file. Explore recorded that sentinel as a follow-up.

## Goals / Non-Goals

**Goals:**
- Land the official LITE extract at the existing seed path without changing catalog keys or filenames.

**Non-Goals:**
- Changing `verifyDatabase`.
- Refreshing IPinfo or the MaxMind dummy.
- New download automation.

## Decisions

- **Copy the official ZIP extract into `seeds/`, do not use the CLI write path.** `go run ./tools/dbdownload -o seeds/...` fails verify. Alternative: change the sentinel to `8.8.8.8` → `US` — rejected this run (requirement Out of scope; name did not move). Alternative: skip the LITE refresh — rejected; that is the only file that measured newer.

- **Prove the extract before overwrite.** SHA256 of the temp extract must match the measured official file (`D23BCFAAAC2FD18135F954D2D3223C8542B65D6B0DDFD99912F18BB00CCFB1BE` as of 2026-09-15) or a freshly downloaded official BIN that still maps `8.8.8.8` → `US` and `1.1.1.1` → `AU`. Then copy onto `seeds/IP2LOCATION-LITE-DB1.IPV6.BIN`.

## Risks / Trade-offs

- [Risk] A later `go generate` still fails on a valid LITE. → Mitigation: `knowledge/debt/2026-09-15-dbdownload-verify-1-1-1-1.md`.
- [Risk] Pinned lookups drift. → Mitigation: run existing package tests that already expect `8.8.8.8` US and `1.1.1.1` AU.

## Migration Plan

Replace the committed BIN. Rollback is revert that file. No Config or deploy step.
