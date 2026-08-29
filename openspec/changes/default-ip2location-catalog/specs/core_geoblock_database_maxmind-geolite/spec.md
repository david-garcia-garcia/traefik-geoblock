## MODIFIED Requirements

### Requirement: Dummy Country MMDB is the default seed
When the bound catalog `path` is empty, the MaxMind provider SHALL open the committed `GeoIP2-Country-Test.mmdb` snapshot (or the newest dated file for the bound catalog key in `databaseAutoUpdateDir` when that directory already has one). Empty `maxmind_source` SHALL bind reserved `default_geolite`. Plugin creation MUST succeed without a dated file when the dummy seed exists. The committed file SHALL be MaxMind’s official dummy Country fixture, not a live GeoLite download.

#### Scenario: Empty pointer binds default_geolite
- **WHEN** `databaseProvider` is `maxmind` and `maxmind_source` is empty and no dated catalog file exists
- **THEN** plugin creation binds `default_geolite`
- **AND** opens the committed `GeoIP2-Country-Test.mmdb` until a dated file exists

#### Scenario: Auto-update dir wins over seed
- **WHEN** `maxmind_source` is `geolite` and `databaseAutoUpdateDir` contains a dated `YYYYMMDD_geolite` MMDB
- **THEN** plugin creation opens that dated file instead of the bundled dummy
