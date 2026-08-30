## Purpose

Defines the geo-database provider the plugin uses so initialization, country lookup, and auto-update are not tied to one vendor at the plugin boundary. This change implements IP2Location, IPinfo Lite, and MaxMind / GeoLite2.

## Requirements

### Requirement: Plugin looks up country through a provider
When `mode` is `enrich` or `enrichandblock`, the plugin SHALL obtain the country for an IP from a DatabaseProvider and SHALL write it to `countryHeader`. It MUST NOT call a vendor SDK type from the request path. The block stage SHALL use the `countryHeader` value for country allow/block, not the lookup `Record` directly. A failed lookup SHALL surface as an error to the existing `banIfError` / allow-on-error behavior. When `mode` is `block` or `disabled`, the plugin MUST NOT look up through a DatabaseProvider.

#### Scenario: Public IP lookup
- **WHEN** `mode` is `enrichandblock` and the plugin checks a public IP
- **THEN** it asks the provider for a country code
- **AND** it writes that code to `countryHeader`
- **AND** country allow/block reads `countryHeader`

#### Scenario: Lookup error
- **WHEN** the provider returns an error for an IP
- **THEN** the plugin applies the existing `banIfError` rule for that IP

#### Scenario: Block mode has no lookup
- **WHEN** `mode` is `block`
- **THEN** the plugin does not call DatabaseProvider Lookup

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
IP2Location download pointers SHALL live on Traefik Config as `ip2location_source_geo` and `ip2location_source_asn`. The plugin MUST pass those fields to the IP2Location provider only. File location SHALL be the catalog `path` and dated files as specified in `core_geoblock_database_url-download`. Empty `ip2location_source_geo` SHALL bind the reserved catalog key `default_ip2location`. Config SHALL NOT expose `ip2location_databaseFilePath`, `ip2location_asnDatabaseFilePath`, or unprefixed `databaseFilePath`.

#### Scenario: Pointer reaches IP2Location only
- **WHEN** `databaseProvider` is `ip2location` and `ip2location_source_geo` names a catalog entry whose `path` is an existing BIN
- **THEN** plugin creation opens that file as the geo database
- **AND** unused IPinfo and MaxMind pointers are ignored

#### Scenario: Empty geo pointer uses default catalog
- **WHEN** `databaseProvider` is `ip2location` and `ip2location_source_geo` is empty
- **THEN** plugin creation binds `default_ip2location`
- **AND** opens that source (dated file, catalog path, or bundled geo BIN)

#### Scenario: Empty MaxMind pointer uses default geolite catalog
- **WHEN** `databaseProvider` is `maxmind` and `maxmind_source` is empty
- **THEN** plugin creation binds `default_geolite`
- **AND** opens that source (dated file, catalog path, or bundled dummy Country MMDB)

### Requirement: Provider implementations are isolated
Vendor Lookup SHALL live in that vendor’s provider package. Open and hot-swap SHALL live in one shared format-wrapper package (`pkg/dbwrappers`), not copied into each vendor package. HTTP GET, archive unpack, file-date read, lock, ticker, dated write, and file-location Resolve SHALL stay one shared source component (`pkg/dbsource`). A later vendor MUST be addable by implementing DatabaseProvider, adding a `databaseProvider` branch, pointing at a catalog entry, and using the BIN or MMDB format wrapper. The plugin MUST NOT type-assert a format wrapper to read provider state.

#### Scenario: Implemented vendors
- **WHEN** this change ships
- **THEN** the IP2Location provider exists
- **AND** the IPinfo Lite provider exists
- **AND** the MaxMind provider exists

#### Scenario: Plugin does not use a wrapper type
- **WHEN** the plugin looks up a public IP
- **THEN** it calls DatabaseProvider Lookup only
- **AND** it does not type-assert a BIN or MMDB wrapper

### Requirement: Open and hot-swap are per format
The format-wrapper package SHALL expose one BIN wrapper and one MMDB wrapper. IPinfo and MaxMind SHALL open through the MMDB wrapper. IP2Location SHALL open geo and ASN through the BIN wrapper (two instances). The same wrapper configuration SHALL share one open file and one download ticker. Closing a DatabaseProvider MUST NOT close that shared wrapper.

#### Scenario: IPinfo and MaxMind share the MMDB wrapper
- **WHEN** `databaseProvider` is `ipinfo` or `maxmind` and plugin creation opens the catalog or bundled MMDB
- **THEN** that file is opened through the shared MMDB wrapper
- **AND** the vendor package only maps lookup fields onto Record

#### Scenario: IP2Location uses two BIN wrappers
- **WHEN** `databaseProvider` is `ip2location` and geo and ASN files are resolved
- **THEN** geo and ASN are opened through the shared BIN wrapper
- **AND** the IP2Location provider maps geo Lookup plus ASN onto one Record

#### Scenario: Same config shares one wrapper
- **WHEN** two plugin instances are created with the same provider config
- **THEN** both Lookups succeed
- **AND** closing one DatabaseProvider does not make the other Lookup fail
