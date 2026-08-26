## Purpose

Defines the geo-database provider the plugin uses so initialization, country lookup, and auto-update are not tied to one vendor at the plugin boundary. This change ships only IP2Location.

## ADDED Requirements

### Requirement: Plugin looks up country through a provider
The plugin SHALL obtain the country for an IP from a DatabaseProvider. It MUST NOT call an IP2Location SDK type from the request path. A failed lookup SHALL surface as an error to the existing `banIfError` / allow-on-error behavior.

#### Scenario: Public IP lookup
- **WHEN** the plugin checks a public IP
- **THEN** it asks the provider for a country code
- **AND** uses that code in the existing country allow/block rules

#### Scenario: Lookup error
- **WHEN** the provider returns an error for an IP
- **THEN** the plugin applies the existing `banIfError` rule for that IP

### Requirement: Provider is selected through config
Creating the plugin SHALL open the DatabaseProvider named by `databaseProvider`. Empty `databaseProvider` SHALL default to `ip2location`. An unknown value SHALL fail plugin creation. Only `ip2location` is implemented.

#### Scenario: Default provider
- **WHEN** the plugin is created with an empty `databaseProvider`
- **THEN** the DatabaseProvider implementation is IP2Location

#### Scenario: Explicit IP2Location
- **WHEN** the plugin is created with `databaseProvider` set to `ip2location`
- **THEN** the DatabaseProvider implementation is IP2Location

#### Scenario: Unknown provider
- **WHEN** the plugin is created with `databaseProvider` set to a value other than `ip2location`
- **THEN** plugin creation fails

### Requirement: IP2Location settings are vendor-prefixed
IP2Location-only settings SHALL live on Traefik Config as `ip2location_databaseFilePath`, `ip2location_databaseAutoUpdate`, `ip2location_databaseAutoUpdateDir`, `ip2location_databaseAutoUpdateToken`, and `ip2location_databaseAutoUpdateCode`. The plugin MUST pass those fields to the IP2Location provider only. Token `file=` behavior SHALL stay as specified in `core_geoblock_database_token-download-file`.

#### Scenario: Auto-update enabled
- **WHEN** `ip2location_databaseAutoUpdate` is true and a directory is set
- **THEN** the IP2Location provider performs the existing init-from-dir and background update behavior

### Requirement: IP2Location implementation is isolated
The IP2Location open, BIN version read, download, extract, and hot-swap SHALL live in the IP2Location provider package. A later vendor MUST be addable by implementing DatabaseProvider and adding a branch on `databaseProvider` without changing allow/block rules. The plugin MUST NOT type-assert a concrete vendor wrapper to read provider state.

#### Scenario: Second vendor is not required
- **WHEN** this change ships
- **THEN** only the IP2Location provider exists
- **AND** MaxMind is not implemented
