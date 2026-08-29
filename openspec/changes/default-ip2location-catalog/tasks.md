## 1. Catalog default and pointer bind

- [ ] 1.1 In `New`, insert `default_ip2location` (free LITE ZIP, `bin`/`zip`) when the operator did not define that key; keep an operator row
- [ ] 1.2 Bind empty `ip2location_source_geo` to `default_ip2location`
- [ ] 1.3 Missing bound catalog key: WARN and treat as empty (geo → default catalog; IPinfo/MaxMind → bundled seed; ASN → no ASN)

## 2. Dir fallback and type check

- [ ] 2.1 Empty `databaseAutoUpdateDir` with a bound URL: WARN and use `filepath.Join(os.TempDir(), "traefik-geoblock")`
- [ ] 2.2 Fail `New` when a bound pointer’s `databaseType` does not match the provider (`bin` vs `mmdb`)

## 3. Tests and docs

- [ ] 3.1 Replace `MissingPointerKeyFails` and URL-without-dir fail tests with WARN + start + fallback
- [ ] 3.2 Add tests: default catalog inserted, operator row kept, empty geo pointer binds default, type mismatch fails
- [ ] 3.3 Document reserved `default_ip2location` in `README.md` and `knowledge/devdocs/core_geoblock_database_source.md`
