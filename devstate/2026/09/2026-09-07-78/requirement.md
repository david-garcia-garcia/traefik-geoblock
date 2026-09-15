# Requirement
IssueKey: 2026-09-07-78

## Problem
When an enabled IP2Location BIN source answers a lookup miss with `country_short` `-`, the plugin writes `-` on the country header instead of `UnknownCountryAlias` (`XX`). Combined catalogs cannot merge a later source's country because `FillEmpty` treats `-` as non-empty, and the hop marks the header written so later hops cannot replace it. Operators who list `XX` in allow/block lists therefore miss BIN-only misses even though mmdb-only misses already get `XX`.

## Current (code)
- `pkg/dbwrappers/bin.go` — `binColumn` returns `rec.Country_short` raw for `country_short`; other BIN columns route through `usableMeta`, which maps `-` to `""`.
- `pkg/dbwrappers/bin.go` — `usableMeta` converts empty, `-`, unavailable, and invalid vendor strings to `""`.
- `pkg/dbprovider/provider.go` — `FillEmpty` copies `src.Country` only when `r.Country == ""`.
- `pkg/dbprovider/combine.go` — merged lookup uses `FillEmpty` per source in catalog order.
- `pkg/geoblock/plugin.go` — `UnknownCountryAlias` is `"XX"`; `writePublicLookupHeaders` writes `XX` when `rec.Country == ""` without setting `written`, but skips that path when country is non-empty (including `-`).
- `pkg/geoblock/config.go` — `sourceEnabled` enables catalog rows when `Enabled` is omitted; default BIN row stays enabled.
- `pkg/geoblock/plugin_mode_test.go` — subtest `a bin source answers - for an unknown address, so XX does not apply` expects header `-` for `203.0.113.7` on a BIN-only catalog.

## Desired
1. Route BIN `country_short` through `usableMeta` (same as other BIN columns) so Combined sees empty country on a vendor miss and later sources can fill.
2. After the hop's merged lookup, when country is still empty, `writePublicLookupHeaders` continues to write `XX` without marking written (already implemented; becomes reachable for BIN-only misses once step 1 applies).
3. Update tests that assert `-` on the country header for BIN unknowns to expect `XX` and merged-catalog fallback behavior per the ticket.

## Affected
- `pkg/dbwrappers/bin.go` (`binColumn` / `usableMeta`)
- `pkg/geoblock/plugin_mode_test.go` (BIN `-` subtest and related expectations)
- Possibly other tests referencing BIN `-` on country headers

## Out of scope
- Emitting `XX` directly from the BIN wrapper (Combined would treat it as resolved and lock the hop).
- Changing `banIfError` semantics for lookup errors or empty/null headers.
- Altering `PRIVATE` fail-open behavior from #76.
- Revisiting non-BIN IP2Location column handling (already through `usableMeta`).

## Unknowns
- Whether any integration or docker tests outside `plugin_mode_test.go` assert `-` on BIN country misses.

## Tensions
- Issue text references tree at `a3da77f` (pre-merge #76); `origin/master` at `a20cd86` now includes #76 (`UnknownCountryAlias`, `writePublicLookupHeaders`, `usableMeta` on non-country BIN columns) — the gap described in the issue remains on master for `country_short`.
- #76 `design.md` explicitly deferred routing `country_short` through `usableMeta`; this ticket is the intentional follow-up called out as a non-goal there.
