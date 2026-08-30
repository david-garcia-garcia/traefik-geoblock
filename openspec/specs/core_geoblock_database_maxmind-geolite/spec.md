## Purpose

Defines MaxMind GeoLite2 / GeoIP2 as a selectable geo database: GeoIP2 nested country lookup, a committed official dummy Country MMDB so plugin creation works without a download, and catalog-pointer auto-update as specified in `core_geoblock_database_url-download`.

## Requirements

### Requirement: MaxMind settings are vendor-prefixed
MaxMind-only settings SHALL live on a `databaseSources` row with `vendor` `maxmind`. The plugin MUST pass that row to the MaxMind Lookup only. File location SHALL be the catalog `path`, `defaultFile`, and dated files as specified in `core_geoblock_database_url-download`. MaxMind MUST NOT use IP2Location `file=` query codes or a vendor-built permalink from `accountId:licenseKey`. Config SHALL NOT expose `maxmind_source` or `maxmind_databaseFilePath`.

#### Scenario: Catalog row reaches MaxMind only
- **WHEN** an enabled row has `vendor` `maxmind` and `path` is an existing GeoIP2-schema MMDB
- **THEN** plugin creation opens that file as the MaxMind database
- **AND** no IP2Location `file=` download is used

### Requirement: Dummy Country MMDB is the default seed
When the MaxMind row's catalog `path` is empty, the MaxMind Lookup SHALL open the committed `GeoIP2-Country-Test.mmdb` snapshot named by that row's `defaultFile` (or the newest dated file for that catalog key). The shipped `default_maxmind` row SHALL set `defaultFile` to `GeoIP2-Country-Test.mmdb` and SHALL be disabled. Empty Config MUST NOT bind `default_geolite` unless the operator enables that row. Plugin creation MUST succeed without a dated file when the dummy seed exists and that row is enabled. The committed file SHALL be MaxMind's official dummy Country fixture, not a live GeoLite download.

#### Scenario: Enabled dummy seed opens the fixture
- **WHEN** `default_maxmind` is enabled and no dated catalog file exists
- **THEN** plugin creation opens the committed `GeoIP2-Country-Test.mmdb`

#### Scenario: Auto-update dir wins over seed
- **WHEN** catalog key `geolite` is an enabled MaxMind row and `databaseAutoUpdateDir` contains a dated `YYYYMMDD_geolite` MMDB
- **THEN** plugin creation opens that dated file instead of the bundled dummy

### Requirement: GeoIP2 lookup maps nested country.iso_code
For a public IP found in a GeoIP2 / GeoLite2 Country or City MMDB, the provider SHALL set country used for allow/block to nested `country.iso_code`. It SHALL NOT use IPinfo `country_code` tags. It SHALL use located `country`, not `registered_country`. Country name SHALL come from `country.names` English when present. Continent and city/region SHALL be filled when the file has those maps. ASN and ISP SHALL stay empty on a Country or City file. `requestHeaderEnrich` SHALL write every mapped header; an empty field SHALL be the string `null`.

#### Scenario: Dummy Country test IP
- **WHEN** the MaxMind provider looks up a documented dummy-fixture IP in `GeoIP2-Country-Test.mmdb`
- **THEN** country used for allow/block is the ISO code stored under `country.iso_code`

#### Scenario: Empty enrich fields are written as null
- **WHEN** `requestHeaderEnrich` maps a header to `asn` and the open file is GeoLite2-Country or the dummy Country fixture
- **THEN** that header is set to `null`
