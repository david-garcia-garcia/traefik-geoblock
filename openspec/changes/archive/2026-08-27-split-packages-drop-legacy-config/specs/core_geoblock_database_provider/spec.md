## Purpose

Defines the geo-database provider the plugin uses so initialization, country lookup, and auto-update are not tied to one vendor at the plugin boundary. This change implements IP2Location and IPinfo Lite.

## ADDED Requirements

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
Creating the plugin SHALL open the DatabaseProvider named by `databaseProvider`. Empty `databaseProvider` SHALL default to `ip2location`. Implemented values SHALL be `ip2location` and `ipinfo`. An unknown value SHALL fail plugin creation.

#### Scenario: Default provider
- **WHEN** the plugin is created with an empty `databaseProvider`
- **THEN** the DatabaseProvider implementation is IP2Location

#### Scenario: Explicit IP2Location
- **WHEN** the plugin is created with `databaseProvider` set to `ip2location`
- **THEN** the DatabaseProvider implementation is IP2Location

#### Scenario: Explicit IPinfo
- **WHEN** the plugin is created with `databaseProvider` set to `ipinfo`
- **THEN** the DatabaseProvider implementation is IPinfo Lite

#### Scenario: Unknown provider
- **WHEN** the plugin is created with `databaseProvider` set to a value other than `ip2location` or `ipinfo`
- **THEN** plugin creation fails

### Requirement: IP2Location settings are vendor-prefixed
IP2Location-only settings SHALL live on Traefik Config as `ip2location_databaseFilePath`, `ip2location_databaseAutoUpdate`, `ip2location_databaseAutoUpdateDir`, `ip2location_databaseAutoUpdateToken`, and `ip2location_databaseAutoUpdateCode`. The plugin MUST pass those fields to the IP2Location provider only. Token `file=` behavior SHALL stay as specified in `core_geoblock_database_token-download-file`.

#### Scenario: Auto-update enabled
- **WHEN** `ip2location_databaseAutoUpdate` is true and a directory is set
- **THEN** the IP2Location provider performs the existing init-from-dir and background update behavior

#### Scenario: Deprecated unprefixed aliases
- **WHEN** an unprefixed IP2Location key is set (`databaseFilePath`, `databaseAutoUpdate`, `databaseAutoUpdateDir`, `databaseAutoUpdateToken`, `databaseAutoUpdateCode`) and the matching `ip2location_` key is unset
- **THEN** the plugin copies the unprefixed value onto the prefixed field
- **AND** logs a deprecation warning
- **AND** a set `ip2location_` key wins over its unprefixed alias

### Requirement: Provider implementations are isolated
Vendor open, file version read, download, extract, and hot-swap SHALL live in that vendor’s provider package. A later vendor MUST be addable by implementing DatabaseProvider and adding a branch on `databaseProvider` without changing allow/block rules. The plugin MUST NOT type-assert a concrete vendor wrapper to read provider state. MaxMind SHALL NOT be implemented.

#### Scenario: Implemented vendors
- **WHEN** this change ships
- **THEN** the IP2Location provider exists
- **AND** the IPinfo Lite provider exists
- **AND** MaxMind is not implemented
