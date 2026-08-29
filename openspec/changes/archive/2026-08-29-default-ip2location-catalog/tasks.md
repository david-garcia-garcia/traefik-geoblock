## 1. Catalog default and pointer bind

- [x] 1.1 In `New`, insert `default_ip2location` (free LITE ZIP, `bin`/`zip`) when the operator did not define that key; keep an operator row
- [x] 1.2 Bind empty `ip2location_source_geo` to `default_ip2location`
- [x] 1.3 Missing bound catalog key: WARN and treat as empty (geo → default catalog; MaxMind → `default_geolite`; IPinfo → bundled seed; ASN → no ASN)

## 2. Dir fallback and type check

- [x] 2.1 Empty `databaseAutoUpdateDir` with a bound URL: WARN and use `filepath.Join(os.TempDir(), "traefik-geoblock")`
- [x] 2.2 Fail `New` when a bound pointer’s `databaseType` does not match the provider (`bin` vs `mmdb`)

## 3. Tests and docs

- [x] 3.1 Replace `MissingPointerKeyFails` and URL-without-dir fail tests with WARN + start + fallback
- [x] 3.2 Add tests: default catalog inserted, operator row kept, empty geo pointer binds default, type mismatch fails
- [x] 3.3 Document reserved `default_ip2location` in `README.md` and `knowledge/devdocs/core_geoblock_database_source.md`

## 4. Default GeoLite catalog

- [x] 4.1 In `New`, insert `default_geolite` (unofficial P3TERX Country MMDB, `mmdb`/`none`) when the operator did not define that key; keep an operator row
- [x] 4.2 Bind empty `maxmind_source` to `default_geolite`; do not commit a live GeoLite file
- [x] 4.3 Document reserved `default_geolite` in `README.md` and `knowledge/devdocs/core_geoblock_database_source.md`
