## MODIFIED Requirements

### Requirement: Downloads are a named catalog
Traefik Config SHALL expose `databaseDownloads` as a map of operator-chosen name to `{ url, headers, databaseType, archive, path }`. `headers` is an optional map of HTTP header name to value. `path` is an optional file or directory used as the seed when no dated catalog file exists. The URL MAY include query parameters (including a download token). Catalog keys are not reserved (`geo` and `asn` are allowed as ordinary names). `CreateConfig` SHALL initialize the map.

#### Scenario: Geo-named catalog key is an ordinary name
- **WHEN** `databaseDownloads` contains a key `geo`
- **THEN** plugin creation does not fail because of that key name

#### Scenario: Query token is accepted
- **WHEN** a catalog entry `url` is an https URL that includes a `token` query parameter
- **THEN** plugin creation does not fail because of that query parameter
- **AND** the plugin does not rewrite the URL

#### Scenario: Catalog path is a seed
- **WHEN** `ipinfo_download` is `lite` and `databaseDownloads.lite.path` is set to an existing MMDB and that entry has no URL
- **THEN** plugin creation opens that file
- **AND** no HTTP download is attempted

### Requirement: Providers bind with pointers
The plugin SHALL expose `ip2location_download_geo`, `ip2location_download_asn`, `ipinfo_download`, and `maxmind_download` as optional catalog key names. An empty pointer SHALL NOT download. A non-empty pointer that is not a key in `databaseDownloads` SHALL fail plugin creation. Pointers that do not apply to the selected `databaseProvider` SHALL be ignored. A named catalog `path` SHALL be used only when a pointer names that entry.

#### Scenario: Missing catalog key fails creation
- **WHEN** `databaseProvider` is `ipinfo` and `ipinfo_download` is `lite` and `databaseDownloads` has no `lite` key
- **THEN** plugin creation fails

#### Scenario: Empty pointer keeps the default seed
- **WHEN** all download pointers are empty and a bundled default database file exists
- **THEN** plugin creation succeeds
- **AND** no HTTP download is attempted

#### Scenario: IP2Location ASN pointer
- **WHEN** `databaseProvider` is `ip2location` and `ip2location_download_asn` names a catalog entry with a URL and `databaseAutoUpdateDir` is set
- **THEN** plugin creation succeeds
- **AND** that entry is eligible for download as the ASN file

## ADDED Requirements

### Requirement: Download component resolves the file location
The shared download component SHALL choose the file the provider opens. When a pointer and `databaseAutoUpdateDir` are set, the newest dated `YYYYMMDD_<catalogKey>` file in that directory SHALL win. Else if the bound catalog `path` is an existing file, that file SHALL be used. `path` is an operator file path (typically absolute), not a basename and not a directory to walk. Else the component SHALL search `TRAEFIK_PLUGIN_GEOBLOCK_PATH` for the vendor default filename (including under `seeds/`). Empty `path` and empty `url` SHALL still search that env for the bundled file. Config SHALL NOT expose `ip2location_databaseFilePath`, `ip2location_asnDatabaseFilePath`, `ipinfo_databaseFilePath`, `maxmind_databaseFilePath`, or unprefixed `databaseFilePath`.

#### Scenario: Dated catalog file wins
- **WHEN** a pointer names catalog key `lite` and `databaseAutoUpdateDir` contains a dated `YYYYMMDD_lite` file
- **THEN** plugin creation opens that dated file

#### Scenario: Empty path uses bundled default
- **WHEN** download pointers are empty and `TRAEFIK_PLUGIN_GEOBLOCK_PATH` contains the vendor default filename
- **THEN** plugin creation opens that bundled file

#### Scenario: No seed Config keys
- **WHEN** Config is decoded
- **THEN** there is no `ip2location_databaseFilePath`, `ipinfo_databaseFilePath`, `maxmind_databaseFilePath`, or `databaseFilePath`
