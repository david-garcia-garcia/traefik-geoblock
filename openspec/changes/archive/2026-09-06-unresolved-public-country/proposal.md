## Why

A public IP that the enabled sources ran against and did not resolve keeps the `PRIVATE`
enrich default, and the block stage reads that back as the `allowPrivate` verdict. With
`defaultAllow: false` and `allowPrivate: true` the request is allowed, bypassing both
`allowedCountries` and deny-by-default. `banIfError` does not fire, because a lookup miss
returns an empty `Record` with a nil error and is not an error.

This is a v1.2.0 regression. Before `mode` split enrich from block, country rules read
`Record.Country` directly, so an unresolved public IP was `""` and fell through to
`defaultAllow`. README still documents that outcome: `block:default_allow` is "Unknown
country, strict config", `block:error` is "Database lookup failure", and `allowPrivate` is
"RFC 1918 / loopback".

The `PRIVATE` default predates the block stage reading the header. It was added so
dashboards never saw an empty country; nothing reconciled it with the header becoming a
policy input.

## What Changes

- Lookup writes `XX` on every `country` enrich header when a public hop's lookup returns an
  empty country. `PRIVATE` stays for private and loopback hops.
- `XX` does not mark the country as written, so a later hop that does resolve still wins.
- Country allow/block therefore reaches `defaultAllow` for an unresolved public address,
  restoring the documented `block:default_allow` outcome.
- `XX` is an ISO 3166-1 user-assigned code, so it cannot collide with a real country and can
  be listed in `allowedCountries` / `blockedCountries` to steer unresolved addresses.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_plugin_request-mode`: Lookup SHALL write `XX` for a public hop the enabled
  sources did not resolve, and `XX` SHALL NOT mark the country written.

## Impact

- `pkg/geoblock/plugin.go` — `UnknownCountryAlias`; `writePublicLookupHeaders` splits the
  empty-country case out of the `PRIVATE` early return.
- `pkg/geoblock/plugin_mode_test.go` — decision coverage for an unresolved public IP.
- `pkg/geoblock/plugin_observe_test.go` — `TestEnrichNullSentinel` asserts `XX` on country.
- `README.md` — enrich values, lookup-mode note, settings table row.
- `knowledge/devdocs/core_geoblock_plugin_request-mode.md`,
  `knowledge/devdocs/core_geoblock_database_lookup.md` — Language and How to use.
