# Requirement
IssueKey: 2026-09-16-fixes

## Problem
Untracked `zzz_proof_*` probes documented five product gaps. The human named the correct contract for each. Dest still implements the old behavior for CIDR `/0` vs a more-specific block, corrupt dated startup, block-mode header/IP error paths, and a silent country-header enrich clash.

## Current (code)
- CIDR specificity: `pkg/geoblock/plugin.go` `decide` requires `allowedNetworkLength > 0`. A `/0` allow reports `0`, so the `else` branch lets `allowed` win over a `/32` block. Measured: `TestProofCatchAllAllowDefeatsSpecificBlock` → `pass:allowed_ip_block`. README line “more specific prefix wins” is already the operator contract.
- Empty `bypassHeaders`: `pkg/geoblock/config.go` `Prepare` rejects `TrimSpace == ""` (commit `579856d`). `blockSkipReason` requires `req.Header.Values` non-empty. Config tests already cover this. The proof fails at `newRoute` because Prepare is correct.
- `countryHeader` + non-country enrich: `pkg/geoblock/config.go` `foldCountryHeader` — existing enrich mapping wins. Config test `RequestHeaderEnrichWinsOverCountryHeader` documents it. No warning at load. Measured: `pass:default_allow` when `X-IPCountry` is mapped to `city`.
- `mode=block` Lookup: `pkg/geoblock/plugin.go` `bindDatabase` returns without opening sources when `!ModeLooksUp`. `Lookup` calls `p.db.Lookup` and panics on nil. `ServeHTTP` does not call `Lookup` in block mode. `blockFromHeader` already treats empty/`null` country as `banIfError`. Empty `CountryHeader` still defaults to `X-IPCountry` in `Prepare` for every blocking mode.
- No client IP: `pkg/geoblock/plugin.go` `blockFromHeader` loops `remoteIPs`; empty slice sets `pass:none` and returns false (pass). Measured: `TestProofNoIPsPassesDespiteDefaultDeny` → 418 / `pass:none` with `defaultAllow=false` and `banIfError=true`.
- Corrupt dated file: `pkg/dbsource/resolve.go` `Resolve` returns `Latest` with no open/validate. `pkg/dbwrappers/lifecycle.go` `initialize` / `publish` then fail on the dated garbage. `seedFirst` only uses `BundledFile` (`defaultFile`), not catalog `Path`. Measured: BIN/MMDB `new*` and root `New` refuse to start with a valid seed `Path` present.

## Desired
- More specific CIDR wins: `/32` `blockedIPBlocks` beats `0.0.0.0/0` `allowedIPBlocks` for `8.8.8.8`.
- Empty bypass: keep today’s Prepare reject; delete or rewrite the obsolete proof (do not change product).
- Enrich mapping a non-country key onto `countryHeader` stays as today. At plugin load, emit a warning that that header is filled with a non-country value.
- `mode=block` works without catalog. `countryHeader` is mandatory in block mode. A missing country header on the request uses the same path as a lookup error (`banIfError`).
- No extractable client IP is an error: log a warning, then block or pass according to `banIfError`.
- Corrupt dated catalog file: do not fail startup. Open the configured seed, warn, keep-current may retry later. BIN and MMDB the same, including Traefik `New`.
- Replace leftover `zzz_proof_*` with ordinary tests that assert these contracts (or delete when an existing test already does).

## Affected
- `pkg/geoblock/plugin.go` `decide`, `blockFromHeader`, `Lookup` / bind
- `pkg/geoblock/config.go` `Prepare`, `foldCountryHeader`
- `pkg/dbwrappers/lifecycle.go` startup publish / seed fallback
- `pkg/dbsource/resolve.go` (only if validation belongs there)
- `zzz_proof_*` under `pkg/dbwrappers`, `pkg/geoblock`, `pkg/iplookup`, repo root
- README / request-mode usage if operator-facing warn/fallback is documented

## Out of scope
- Vendor `Get_all` on a nil `*ip2loc.DB` (IP2Location panic). Product already avoids that call.
- IPv4/IPv6 family leak (already isolated; helper test passes).
- Re-implementing empty `bypassHeaders` reject.
- Changing the enrich-wins-over-`countryHeader` mapping itself.
- The already-committed presets file split on this branch (not this ask).

## Unknowns
- In `mode=block`, does “`countryHeader` mandatory” reject an omitted field, or is the existing `X-IPCountry` default still enough?
- After a corrupt dated file is skipped, should keep-current keep trying that dated name, or skip it until a newer file appears?
- Should `Lookup` / `CheckAllowed` on a block-mode plugin return a typed error, or is fixing `ServeHTTP` enough?

## Tensions
- Human: `countryHeader` mandatory in block mode. Code: empty `CountryHeader` defaults to `X-IPCountry` for all modes (`Prepare`). Making it required is a breaking load for configs that omit the key and rely on the default.
- Human: missing country header in block mode = lookup error. Code: `blockFromHeader` already bans on empty country when `banIfError` is true. Gap may be only the no-IP path and the exported `Lookup` panic.
- Proof files always `Fatalf` on both success and failure for corrupt dated; they are not contracts. Desired tests must assert seed+warn, not “refused to start”.
