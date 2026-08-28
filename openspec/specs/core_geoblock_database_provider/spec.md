## Purpose

Defines the geo-database provider the plugin uses so initialization, country lookup, and auto-update are not tied to one vendor at the plugin boundary. This change implements IP2Location, IPinfo Lite, and MaxMind / GeoLite2.

## Requirements

### Requirement: Plugin looks up country through a provider
The plugin SHALL obtain the country for an IP from a DatabaseProvider. It MUST NOT call a vendor SDK type from the request path. A failed lookup SHALL surface as an error to the existing `banIfError` / allow-on-error behavior.

#### Scenario: Public IP lookup
- **WHEN** the plugin checks a public IP
- **THEN** it asks the provider for a country code
- **AND** uses that code in the existing country allow/block rules

#### Scenario: Lookup error
- **WHEN** the provider returns an error for an IP
- **THEN** the plugin applies the existing `banIfError` rule for that IP

### Requirement: Provider is selected through config
Creating the plugin SHALL open the DatabaseProvider named by `databaseProvider`. Empty `databaseProvider` SHALL default to `ip2location`. Implemented values SHALL be `ip2location`, `ipinfo`, and `maxmind`. An unknown value SHALL fail plugin creation.

#### Scenario: Default provider
- **WHEN** the plugin is created with an empty `databaseProvider`
- **THEN** the DatabaseProvider implementation is IP2Location

#### Scenario: Explicit IP2Location
- **WHEN** the plugin is created with `databaseProvider` set to `ip2location`
- **THEN** the DatabaseProvider implementation is IP2Location

#### Scenario: Explicit IPinfo
- **WHEN** the plugin is created with `databaseProvider` set to `ipinfo`
- **THEN** the DatabaseProvider implementation is IPinfo Lite

#### Scenario: Explicit MaxMind
- **WHEN** the plugin is created with `databaseProvider` set to `maxmind`
- **THEN** the DatabaseProvider implementation is MaxMind / GeoLite2

#### Scenario: Unknown provider
- **WHEN** the plugin is created with `databaseProvider` set to a value other than `ip2location`, `ipinfo`, or `maxmind`
- **THEN** plugin creation fails

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
