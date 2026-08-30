## Purpose

Defines IPinfo Lite as a selectable geo database: one MMDB for country and ASN, a committed snapshot so plugin creation works without a download, and catalog-pointer auto-update as specified in `core_geoblock_database_url-download`.

## Requirements

### Requirement: IPinfo settings are vendor-prefixed
IPinfo-only settings SHALL live on a `databaseSources` row with `vendor` `ipinfo`. The plugin MUST pass that row to the IPinfo Lookup only. File location SHALL be the catalog `path`, `defaultFile`, and dated files as specified in `core_geoblock_database_url-download`. IPinfo MUST NOT use IP2Location `file=` query codes or a vendor-built `ipinfo_{code}.mmdb?token=` URL. Config SHALL NOT expose `ipinfo_source` or `ipinfo_databaseFilePath`.

#### Scenario: Catalog row reaches IPinfo only
- **WHEN** an enabled row has `vendor` `ipinfo` and `path` is an existing MMDB
- **THEN** plugin creation opens that file as the IPinfo database
- **AND** no IP2Location `file=` download is used

### Requirement: Bundled Lite MMDB is the default seed
When the IPinfo row's catalog `path` is empty, the IPinfo Lookup SHALL open the committed `ipinfo_lite.mmdb` snapshot named by that row's `defaultFile` (or the newest dated file for that catalog key in `databaseAutoUpdateDir` when that directory already has one). The shipped `default_ipinfo` row SHALL set `defaultFile` to `ipinfo_lite.mmdb`. A stale bundled snapshot SHALL still open. Plugin creation MUST succeed without a download URL when a seed file exists and that row is enabled.

#### Scenario: Empty path uses bundled snapshot
- **WHEN** an enabled IPinfo row has empty `path` and no dated catalog file exists
- **AND** `defaultFile` is `ipinfo_lite.mmdb`
- **THEN** plugin creation opens the committed `ipinfo_lite.mmdb`

#### Scenario: Auto-update dir wins over seed
- **WHEN** catalog key `lite` is an enabled IPinfo row and `databaseAutoUpdateDir` contains a dated `YYYYMMDD_lite` MMDB
- **THEN** plugin creation opens that dated file instead of the bundled snapshot

### Requirement: Lite lookup maps country code and ASN fields
For a public IP found in the Lite MMDB, the provider SHALL set country used for allow/block to Lite `country_code`. It SHALL also expose country name (`country`), continent, continent code, ISP from `as_name`, domain from `as_domain`, and ASN from `asn` including any `AS` prefix present in the database. Region and city SHALL be empty. `requestHeaderEnrich` SHALL write every mapped header; an empty field SHALL be the string `null`.

#### Scenario: Known public IP
- **WHEN** the IPinfo provider looks up `8.8.8.8` in the Lite MMDB
- **THEN** country used for allow/block is `US`
- **AND** country name is `United States`
- **AND** ASN is `AS15169`

#### Scenario: Empty region and city are written as null
- **WHEN** `requestHeaderEnrich` maps headers to `region` and `city` and the lookup is IPinfo Lite
- **THEN** those headers are set to `null`
