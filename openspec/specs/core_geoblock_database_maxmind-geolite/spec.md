## Purpose

Defines MaxMind GeoLite2 / GeoIP2 as a selectable geo database: GeoIP2 nested country lookup, a committed official dummy Country MMDB so plugin creation works without a download, and catalog-pointer auto-update as specified in `core_geoblock_database_url-download`.

## Requirements

### Requirement: MaxMind settings are vendor-prefixed
MaxMind-only settings SHALL live on Traefik Config as `maxmind_source`. The plugin MUST pass that field to the MaxMind provider only. File location SHALL be the catalog `path` and dated files as specified in `core_geoblock_database_url-download`. MaxMind MUST NOT use IP2Location `file=` query codes or a vendor-built permalink from `accountId:licenseKey`. Config SHALL NOT expose `maxmind_databaseFilePath`.

#### Scenario: Pointer reaches MaxMind only
- **WHEN** `databaseProvider` is `maxmind` and `maxmind_source` names a catalog entry whose `path` is an existing GeoIP2-schema MMDB
- **THEN** plugin creation opens that file as the MaxMind database
- **AND** no IP2Location `file=` download is used

### Requirement: Dummy Country MMDB is the default seed
When the bound catalog `path` is empty, the MaxMind provider SHALL open the committed `GeoIP2-Country-Test.mmdb` snapshot (or the newest dated file for the bound catalog key in `databaseAutoUpdateDir` when that directory already has one). Empty `maxmind_source` SHALL bind reserved `default_geolite`. Plugin creation MUST succeed without a dated file when the dummy seed exists. The committed file SHALL be MaxMind’s official dummy Country fixture, not a live GeoLite download.

#### Scenario: Empty pointer binds default_geolite
- **WHEN** `databaseProvider` is `maxmind` and `maxmind_source` is empty and no dated catalog file exists
- **THEN** plugin creation binds `default_geolite`
- **AND** opens the committed `GeoIP2-Country-Test.mmdb` until a dated file exists

#### Scenario: Auto-update dir wins over seed
- **WHEN** `maxmind_source` is `geolite` and `databaseAutoUpdateDir` contains a dated `YYYYMMDD_geolite` MMDB
- **THEN** plugin creation opens that dated file instead of the bundled dummy

### Requirement: GeoIP2 lookup maps nested country.iso_code
For a public IP found in a GeoIP2 / GeoLite2 Country or City MMDB, the provider SHALL set country used for allow/block to nested `country.iso_code`. It SHALL NOT use IPinfo `country_code` tags. It SHALL use located `country`, not `registered_country`. Country name SHALL come from `country.names` English when present. Continent and city/region SHALL be filled when the file has those maps. ASN and ISP SHALL stay empty on a Country or City file. `requestHeaderEnrich` SHALL write every mapped header; an empty field SHALL be the string `null`.

#### Scenario: Dummy Country test IP
- **WHEN** the MaxMind provider looks up a documented dummy-fixture IP in `GeoIP2-Country-Test.mmdb`
- **THEN** country used for allow/block is the ISO code stored under `country.iso_code`

#### Scenario: Empty enrich fields are written as null
- **WHEN** `requestHeaderEnrich` maps a header to `asn` and the open file is GeoLite2-Country or the dummy Country fixture
- **THEN** that header is set to `null`
