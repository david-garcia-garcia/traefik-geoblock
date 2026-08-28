## MODIFIED Requirements

### Requirement: IP2Location settings are vendor-prefixed
IP2Location download pointers SHALL live on Traefik Config as `ip2location_download_geo` and `ip2location_download_asn`. The plugin MUST pass those fields to the IP2Location provider only. File location SHALL be the catalog `path` and dated files as specified in `core_geoblock_database_url-download`. Config SHALL NOT expose `ip2location_databaseFilePath`, `ip2location_asnDatabaseFilePath`, or unprefixed `databaseFilePath`.

#### Scenario: Pointer reaches IP2Location only
- **WHEN** `databaseProvider` is `ip2location` and `ip2location_download_geo` names a catalog entry whose `path` is an existing BIN
- **THEN** plugin creation opens that file as the geo database
- **AND** unused IPinfo and MaxMind pointers are ignored

#### Scenario: Empty geo pointer uses bundled default
- **WHEN** `databaseProvider` is `ip2location` and `ip2location_download_geo` is empty and the default geo BIN exists
- **THEN** plugin creation opens that bundled geo database

### Requirement: Provider implementations are isolated
Vendor Lookup open and hot-swap SHALL live in that vendor’s provider package. HTTP GET, archive unpack, file-date read, lock, ticker, dated write, and file-location Resolve SHALL be one shared download component, not copied into each vendor package. A later vendor MUST be addable by implementing DatabaseProvider, adding a `databaseProvider` branch, and pointing at a catalog entry. The plugin MUST NOT type-assert a concrete vendor wrapper to read provider state.

#### Scenario: Implemented vendors
- **WHEN** this change ships
- **THEN** the IP2Location provider exists
- **AND** the IPinfo Lite provider exists
- **AND** the MaxMind provider exists
