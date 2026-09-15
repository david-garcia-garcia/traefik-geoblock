# Treat BIN country `-` as empty so catalog fallback and plugin `XX` apply

Follow-up to [#76](https://github.com/david-garcia-garcia/traefik-geoblock/pull/76) (`a3da77f`). Grounded on that PR's tree, not `master`.

## Existing problem

[#76](https://github.com/david-garcia-garcia/traefik-geoblock/pull/76) introduced `UnknownCountryAlias` (`XX`) as the plugin's country when a **public** hop's lookup leaves `Record.Country` empty. `writePublicLookupHeaders` writes `XX` on country enrich headers and does **not** mark the country written, so a later hop that resolves still wins. Empty/`null` on the header remains the `banIfError` path; `PRIVATE` remains private/loopback.

That empty-country path does **not** run when an enabled BIN source answers. `binColumn` returns `country_short` **raw**. Every other BIN column maps IP2Location's `-` to `""` through `usableMeta`. Combined only copies a field when the current value is `""` (`FillEmpty`). `-` is not empty, so:

1. The plugin writes `-` on `countryHeader`, not `XX`.
2. A later catalog source (MMDB / IPinfo) cannot fill country for that hop.
3. Because country is non-empty, that hop marks the header written; a later hop cannot replace `-`.

This is asserted on the PR: `TestMode_UnresolvedPublicIPFollowsDefaultAllow` subtest "a bin source answers - for an unknown address, so XX does not apply" expects `X-Ipcountry` `-` for `203.0.113.7` on a BIN catalog.

The reserved `default_ip2location` row is enabled when `Enabled` is omitted (`sourceEnabled`). Default catalogs therefore keep `-` for a BIN miss. Verdicts already follow `defaultAllow` (`-` is not in `allowedCountries`). The split is the **value**: BIN miss = `-`, mmdb miss = `XX`. `allowedCountries: [XX]` does not cover BIN misses.

[#76](https://github.com/david-garcia-garcia/traefik-geoblock/pull/76) `design.md` listed routing `country_short` through `usableMeta` as a **non-goal**: it must not land in the same change as the `PRIVATE` fail-open, because it would hit every IP2Location catalog.

| Value | Who | Meaning on #76 |
| --- | --- | --- |
| `-` | IP2Location `country_short` | This BIN source has no country |
| `""` | Combined / `usableMeta` | Field still empty; next source may fill |
| `XX` | Plugin `UnknownCountryAlias` | After lookup, still no country; not `PRIVATE`, not `banIfError` |

Paths on `a3da77f`: `pkg/dbwrappers/bin.go` (`binColumn` / `usableMeta`), `pkg/dbprovider/combine.go` + `provider.go` (`FillEmpty`), `pkg/geoblock/plugin.go` (`UnknownCountryAlias`, `writePublicLookupHeaders`), `pkg/geoblock/config.go` (`sourceEnabled`, `insertReservedCatalog`), `pkg/geoblock/plugin_mode_test.go` (BIN `-` subtest).

## Proposed solution

Two owners, in this order. Do **not** have BIN emit `XX` (Combined would treat it as a resolved country, skip later sources, and lock the hop).

1. **Catalog merge** — treat BIN `-` as nothing found: run `country_short` through `usableMeta` (same as the other BIN columns) so Combined sees `""` and a later source can fill country.
2. **Plugin, after that hop's merged lookup** — if country is still empty, write `XX` without marking written. `writePublicLookupHeaders` already does this; it starts applying to BIN-only misses once step 1 leaves country empty.

Effects on a default catalog (enabled BIN):

- Header for a BIN miss changes `-` → `XX`.
- `allowedCountries: [XX]` / `blockedCountries: [XX]` then cover BIN misses as well as mmdb misses.
- Deny-by-default verdicts for a BIN-only miss stay `defaultAllow` (today `-` already misses country maps).
- Merged catalogs can take a later source's country when BIN has `-`.
- Hop chains stay "first **resolved** public country wins": empty/`XX` from this hop does not lock.

Keep `banIfError` on lookup **errors** and on missing/empty/`null` headers. A vendor miss is not an error.
