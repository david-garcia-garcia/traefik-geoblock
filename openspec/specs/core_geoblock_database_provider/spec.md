## Purpose

Defines the geo-database provider the plugin uses so initialization, country lookup, and auto-update are not tied to one vendor at the plugin boundary. This change implements IP2Location, IPinfo Lite, and MaxMind / GeoLite2.

## Requirements

### Requirement: Plugin looks up country through a provider
When `mode` is `enrich` or `enrichandblock`, the plugin SHALL obtain the country for an IP from the merged enabled catalog sources and SHALL write it to `countryHeader`. It MUST NOT call a vendor SDK type from the request path. The block stage SHALL use the `countryHeader` value for country allow/block, not the lookup `Record` directly. A failed lookup SHALL surface as an error to the existing `banIfError` / allow-on-error behavior. When `mode` is `block` or `disabled`, the plugin MUST NOT look up through catalog sources.

#### Scenario: Public IP lookup
- **WHEN** `mode` is `enrichandblock` and the plugin checks a public IP
- **THEN** it asks the merged sources for a country code
- **AND** it writes that code to `countryHeader`
- **AND** country allow/block reads `countryHeader`

#### Scenario: Lookup error
- **WHEN** every enabled source returns an error for an IP
- **THEN** the plugin applies the existing `banIfError` rule for that IP

#### Scenario: Block mode has no lookup
- **WHEN** `mode` is `block`
- **THEN** the plugin does not call source Lookup

### Requirement: Provider is selected through config
Creating the plugin SHALL open enabled `databaseSources` rows. Config MUST NOT expose `databaseProvider`. Implemented `vendor` values SHALL be `ip2location`, `ip2location-asn`, `ipinfo`, and `maxmind`. An unknown `vendor` on an enabled row SHALL fail plugin creation.

#### Scenario: Default sources
- **WHEN** the plugin is created with empty `databaseSources` and `mode` is `enrichandblock`
- **THEN** the open source is `default_ip2location`

#### Scenario: Explicit IPinfo row
- **WHEN** an enabled catalog row has `vendor` `ipinfo`
- **THEN** that row is opened with the IPinfo field map

#### Scenario: Explicit MaxMind row
- **WHEN** an enabled catalog row has `vendor` `maxmind`
- **THEN** that row is opened with the MaxMind field map

#### Scenario: Unknown vendor
- **WHEN** an enabled catalog row has `vendor` `no-such-vendor`
- **THEN** plugin creation fails

### Requirement: Provider implementations are isolated
Vendor field maps SHALL live on the format wrapper (`BINSource`, `ASNSource`, `IPinfo`, `GeoIP2`). Open and hot-swap SHALL live in `pkg/dbwrappers`. HTTP GET and Resolve SHALL stay in `pkg/dbsource`. Shipped seed filenames and reserved download URLs SHALL live on catalog insert. A later vendor MUST be addable by adding a wrapper Lookup for a new `vendor` value and a catalog row. The tree MUST NOT keep a vendor package that only wraps Lookup. The plugin MUST NOT type-assert a format wrapper to read source state.

#### Scenario: Implemented vendors
- **WHEN** this change ships
- **THEN** IP2Location geo and ASN Lookups exist
- **AND** the IPinfo Lookup exists
- **AND** the MaxMind Lookup exists

#### Scenario: Plugin does not use a wrapper type
- **WHEN** the plugin looks up a public IP
- **THEN** it calls merged source Lookup only
- **AND** it does not type-assert a BIN or MMDB wrapper

### Requirement: Open and hot-swap are per format
The format-wrapper package SHALL expose one BIN wrapper and one MMDB wrapper. IPinfo and MaxMind SHALL open through the MMDB wrapper. IP2Location geo and ASN SHALL open through the BIN wrapper (one instance per catalog row). The same wrapper configuration SHALL share one open file and one download ticker. Closing the merged Provider MUST NOT close that shared wrapper.

#### Scenario: IPinfo and MaxMind share the MMDB wrapper
- **WHEN** an enabled row has `vendor` `ipinfo` or `maxmind`
- **THEN** that file is opened through the shared MMDB wrapper

#### Scenario: IP2Location uses one BIN wrapper per row
- **WHEN** enabled rows have `vendor` `ip2location` and `ip2location-asn`
- **THEN** each row is opened through the shared BIN wrapper
- **AND** merge maps geo fields plus ASN onto one Record

#### Scenario: Same config shares one wrapper
- **WHEN** two plugin instances are created with the same enabled catalog rows
- **THEN** both Lookups succeed
- **AND** closing one merged Provider does not make the other Lookup fail
