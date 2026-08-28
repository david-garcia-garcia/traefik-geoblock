## MODIFIED Requirements

### Requirement: IP2Location settings are vendor-prefixed
IP2Location-only file paths SHALL live on Traefik Config as `ip2location_databaseFilePath` and `ip2location_asnDatabaseFilePath`. IP2Location download pointers SHALL be `ip2location_download_geo` and `ip2location_download_asn`. The plugin MUST pass those fields to the IP2Location provider only. HTTP download SHALL use the catalog and `databaseAutoUpdateDir` as specified in `core_geoblock_database_url-download`.

#### Scenario: Prefixed seed path
- **WHEN** `databaseProvider` is `ip2location` and `ip2location_databaseFilePath` is set to an existing BIN and download pointers are empty
- **THEN** plugin creation opens that file as the geo database

#### Scenario: Deprecated unprefixed file-path alias
- **WHEN** unprefixed `databaseFilePath` is set and `ip2location_databaseFilePath` is unset
- **THEN** the plugin copies the unprefixed value onto the prefixed field
- **AND** logs a deprecation warning
- **AND** a set `ip2location_databaseFilePath` wins over the alias

### Requirement: Provider implementations are isolated
Vendor Lookup open and hot-swap SHALL live in that vendor’s provider package. HTTP GET, archive unpack, file-date read, lock, ticker, and dated write SHALL be one shared download component, not copied into each vendor package. A later vendor MUST be addable by implementing DatabaseProvider, adding a `databaseProvider` branch, and pointing at a catalog entry. The plugin MUST NOT type-assert a concrete vendor wrapper to read provider state.

#### Scenario: Implemented vendors
- **WHEN** this change ships
- **THEN** the IP2Location provider exists
- **AND** the IPinfo Lite provider exists
- **AND** the MaxMind provider exists
