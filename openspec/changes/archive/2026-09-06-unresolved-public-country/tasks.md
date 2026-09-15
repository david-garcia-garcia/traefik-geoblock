## 1. Lookup

- [x] 1.1 Add `UnknownCountryAlias`.
- [x] 1.2 Split the empty-country case out of the `PRIVATE` early return in
      `writePublicLookupHeaders`; write `XX` on every `country` enrich header without
      marking the country written.

## 2. Tests

- [x] 2.1 `defaultAllow: false` blocks an unresolved public IP and the header is `XX`.
- [x] 2.2 `defaultAllow: true` still allows it.
- [x] 2.3 A private hop keeps `PRIVATE` and follows `allowPrivate`.
- [x] 2.4 A later hop that resolves still wins the country header.
- [x] 2.5 An unresolved hop before a private hop still blocks.
- [x] 2.6 `CheckFirstNonePrivate` blocks an unresolved public hop after a private one.
- [x] 2.7 A `bin` source answers `-` for an unknown address, so `XX` does not apply.
- [x] 2.8 Every header mapped to `country` receives `XX`.
- [x] 2.9 `TestEnrichNullSentinel` asserts `XX` on country and `null` on the other keys.

## 3. Docs

- [x] 3.1 README: enrich values, the lookup-mode `countryHeader` note, settings table row.
- [x] 3.2 Usage packets `knowledge/devdocs/core_geoblock_plugin_request-mode.md` and
      `knowledge/devdocs/core_geoblock_database_lookup.md`: Language and How to use.
