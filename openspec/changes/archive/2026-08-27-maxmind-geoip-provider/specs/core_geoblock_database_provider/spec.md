## Purpose

Defines the geo-database provider the plugin uses so initialization, country lookup, and auto-update are not tied to one vendor at the plugin boundary. This change adds MaxMind as a third implementation.

## MODIFIED Requirements

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

### Requirement: Provider implementations are isolated
Vendor open, file version read, download, extract, and hot-swap SHALL live in that vendor’s provider package. A later vendor MUST be addable by implementing DatabaseProvider and adding a branch on `databaseProvider` without changing allow/block rules. The plugin MUST NOT type-assert a concrete vendor wrapper to read provider state.

#### Scenario: Implemented vendors
- **WHEN** this change ships
- **THEN** the IP2Location provider exists
- **AND** the IPinfo Lite provider exists
- **AND** the MaxMind provider exists
