## 1. Prepare

- [x] 1.1 Remove `checkCountryHeaderBridge` from `Prepare`.
- [x] 1.2 Keep `foldCountryHeader` so `countryHeader` is still written when enrich only names the extra header.

## 2. Tests

- [x] 2.1 `Prepare` succeeds when enrich maps `country` onto a header that is not `countryHeader`.
- [x] 2.2 Lookup writes both `countryHeader` and the extra enrich country header.

## 3. Docs

- [x] 3.1 README: extra `country` enrich headers are written; block still reads `countryHeader`.
- [x] 3.2 Usage packet `knowledge/devdocs/core_geoblock_plugin_request-mode.md`: Language and How to use.
