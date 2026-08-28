## MODIFIED Requirements

### Requirement: IPinfo settings are vendor-prefixed
IPinfo-only settings SHALL live on Traefik Config as `ipinfo_source`. The plugin MUST pass that field to the IPinfo provider only. File location SHALL be the catalog `path` and dated files as specified in `core_geoblock_database_url-download`. IPinfo MUST NOT use IP2Location `file=` query codes or a vendor-built `ipinfo_{code}.mmdb?token=` URL. Config SHALL NOT expose `ipinfo_databaseFilePath`.

#### Scenario: Pointer reaches IPinfo only
- **WHEN** `databaseProvider` is `ipinfo` and `ipinfo_source` names a catalog entry whose `path` is an existing MMDB
- **THEN** plugin creation opens that file as the IPinfo database
- **AND** no IP2Location `file=` download is used

### Requirement: Bundled Lite MMDB is the default seed
When the bound catalog `path` is empty (or the IPinfo pointer is empty), the IPinfo provider SHALL open the committed `ipinfo_lite.mmdb` snapshot (or the newest dated file for the `ipinfo_source` catalog key in `databaseAutoUpdateDir` when that directory already has one). A stale bundled snapshot SHALL still open. Plugin creation MUST succeed without a download pointer when a seed file exists.

#### Scenario: Empty path uses bundled snapshot
- **WHEN** `databaseProvider` is `ipinfo` and `ipinfo_source` is empty and no dated catalog file exists
- **THEN** plugin creation opens the committed `ipinfo_lite.mmdb`

#### Scenario: Auto-update dir wins over seed
- **WHEN** `ipinfo_source` is `lite` and `databaseAutoUpdateDir` contains a dated `YYYYMMDD_lite` MMDB
- **THEN** plugin creation opens that dated file instead of the bundled snapshot
