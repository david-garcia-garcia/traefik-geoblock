## Context

See proposal.md — Why. `writePublicLookupHeaders` already writes `XX` when `Record.Country` is empty, without marking written. Combined copies a field only when the current value is `""`. `usableMeta` already maps IP2Location `-` to `""` for every BIN column except `country_short`. That exception is why dest still writes `-` on a default catalog.

#76 listed routing `country_short` through `usableMeta` as a non-goal so it would not land with the PRIVATE fail-open. That fail-open is now on `master`. This change is only the deferred mapping.

Hop IP stays on `IPHeaders` / `iplookup`. Country after merge stays on Combined. `XX` stays on `writePublicLookupHeaders`. BIN MUST NOT emit `XX`.

## Goals / Non-Goals

**Goals:**
- BIN `country_short` `-` is empty on the Record.
- A later catalog source can fill country for that hop.
- After merge, still-empty country writes `XX` without locking the hop.
- `allowedCountries: [XX]` / `blockedCountries: [XX]` cover BIN-only misses.

**Non-Goals:**
- BIN emitting `XX`.
- Changing `banIfError` (a vendor miss is not an error).
- Changing `PRIVATE` or the #76 fail-open.
- Treating MMDB / IPinfo miss sentinels as empty.

## Decisions

- **Empty at the wrapper, `XX` at the plugin.** Same owners as dest. One product line: `country_short` through `usableMeta`.
- **Fold, not a new leaf.** `core_geoblock_database_lookup` owns BIN column copy. `core_geoblock_plugin_request-mode` already requires `XX` when sources returned no country; BIN `-` was the hole.
- **Flip the #76 BIN `-` subtest; add a merged-catalog fallback.** Ticket Desired names both.

## Risks / Trade-offs

- [Operators who scrape `countryHeader` `-` for BIN unknowns] → they see `XX`. That is the fix. README currently documents `-`.
- [Deny-by-default verdicts] → unchanged: `-` already missed country maps; `XX` still misses unless listed.
- [Merged catalogs] → later sources can now fill country when BIN has `-`. That is intended.

## Migration Plan

Ship it. Operators who listed `XX` to cover unresolved addresses now cover default BIN catalogs too. Rollback is revert.
