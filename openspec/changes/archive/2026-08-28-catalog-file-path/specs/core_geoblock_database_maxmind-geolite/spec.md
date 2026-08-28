## MODIFIED Requirements

### Requirement: MaxMind settings are vendor-prefixed
MaxMind-only settings SHALL live on Traefik Config as `maxmind_download`. The plugin MUST pass that field to the MaxMind provider only. File location SHALL be the catalog `path` and dated files as specified in `core_geoblock_database_url-download`. MaxMind MUST NOT use IP2Location `file=` query codes or a vendor-built permalink from `accountId:licenseKey`. Config SHALL NOT expose `maxmind_databaseFilePath`.

#### Scenario: Pointer reaches MaxMind only
- **WHEN** `databaseProvider` is `maxmind` and `maxmind_download` names a catalog entry whose `path` is an existing GeoIP2-schema MMDB
- **THEN** plugin creation opens that file as the MaxMind database
- **AND** no IP2Location `file=` download is used

### Requirement: Dummy Country MMDB is the default seed
When the bound catalog `path` is empty (or the MaxMind pointer is empty), the MaxMind provider SHALL open the committed `GeoIP2-Country-Test.mmdb` snapshot (or the newest dated file for the `maxmind_download` catalog key in `databaseAutoUpdateDir` when that directory already has one). Plugin creation MUST succeed without a download pointer when a seed file exists. The committed file SHALL be MaxMind’s official dummy Country fixture, not a live GeoLite download.

#### Scenario: Empty path uses bundled dummy
- **WHEN** `databaseProvider` is `maxmind` and `maxmind_download` is empty and no dated catalog file exists
- **THEN** plugin creation opens the committed `GeoIP2-Country-Test.mmdb`

#### Scenario: Auto-update dir wins over seed
- **WHEN** `maxmind_download` is `geolite` and `databaseAutoUpdateDir` contains a dated `YYYYMMDD_geolite` MMDB
- **THEN** plugin creation opens that dated file instead of the bundled dummy
