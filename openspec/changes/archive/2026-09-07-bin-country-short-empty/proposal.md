## Why

An enabled IP2Location BIN source answers a lookup miss with `country_short` `-`. Combined treats that as a filled country, so a later catalog source cannot fill, the hop writes `-` on `countryHeader` (not `XX`), and `allowedCountries: [XX]` misses BIN-only misses. #76 deferred routing `country_short` through `usableMeta` on purpose; this change is that follow-up.

## What Changes

- **BREAKING (header value):** a BIN lookup miss writes `XX` on country enrich headers, not `-`.
- BIN `country_short` goes through `usableMeta` like the other BIN columns, so Combined sees empty country and a later source can fill.
- After the hop's merged lookup, empty country still writes `XX` without marking written (`writePublicLookupHeaders` already does this).
- BIN MUST NOT emit `XX` (Combined would treat it as resolved and lock the hop).
- Tests that asserted `-` for a BIN unknown expect `XX`. A merged-catalog case proves a later source can fill country when BIN has `-`.
- README and usage packets drop the "BIN `-` counts as a country" note.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_database_lookup`: BIN `country_short` `-` SHALL be empty on the Record so Combined can fill from a later source.
- `core_geoblock_plugin_request-mode`: a public hop whose merged lookup still has no country SHALL write `XX` even when an enabled BIN row answered `-`. `XX` SHALL NOT mark the country written.

## Impact

- `pkg/dbwrappers/bin.go` — `binColumn` `country_short` through `usableMeta`.
- `pkg/geoblock/plugin_mode_test.go` — flip BIN-miss header assertion; add merged-catalog fallback.
- `README.md` — lookup-mode `countryHeader` note.
- `knowledge/devdocs/core_geoblock_plugin_request-mode.md`, `knowledge/devdocs/core_geoblock_database_lookup.md` — drop BIN `-` as a country.
