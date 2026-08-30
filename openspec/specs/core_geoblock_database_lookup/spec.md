## Purpose

Defines how the plugin looks up geo fields for an IP from enabled catalog sources so initialization, country lookup, and auto-update are not tied to one vendor at the plugin boundary. Enabled catalog rows open as BIN or MMDB wrappers and fill one Record.

## Requirements

### Requirement: Plugin looks up country through merged catalog sources
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

### Requirement: Lookup opens enabled catalog rows
Creating the plugin SHALL open enabled `databaseSources` rows. Config MUST NOT expose `databaseProvider` or `vendor`. An enabled row SHALL open as BIN when `databaseType` is `bin` and as MMDB when `databaseType` is `mmdb`. Lookup SHALL call `LookupRecord(ip, fields)` with that row's Field map.

#### Scenario: Default sources
- **WHEN** the plugin is created with empty `databaseSources` and `mode` is `enrichandblock`
- **THEN** the open source is `default_ip2location`

#### Scenario: Explicit IPinfo row
- **WHEN** an enabled catalog row has `databaseType` `mmdb` and `fieldsPreconfigured` `ipinfo_lite`
- **THEN** that row is opened through the MMDB wrapper with the IPinfo Lite Field map

#### Scenario: Explicit MaxMind row
- **WHEN** an enabled catalog row has `databaseType` `mmdb` and `fieldsPreconfigured` `maxmind_country`
- **THEN** that row is opened through the MMDB wrapper with the MaxMind Country Field map

### Requirement: Field maps live on the format wrapper
Field maps and named presets SHALL live in `pkg/dbwrappers`. Open and hot-swap SHALL live in `pkg/dbwrappers`. HTTP GET and Resolve SHALL stay in `pkg/dbsource`. Shipped seed filenames and reserved download URLs SHALL live on catalog insert. The tree MUST NOT keep a vendor package or hidden IPinfo/GeoIP2 record types that only wrap Lookup. The plugin MUST NOT type-assert a format wrapper to read source state.

#### Scenario: Implemented formats
- **WHEN** this change ships
- **THEN** BIN `LookupRecord` exists
- **AND** MMDB `LookupRecord` exists

#### Scenario: Plugin does not use a wrapper type
- **WHEN** the plugin looks up a public IP
- **THEN** it calls merged source Lookup only
- **AND** it does not type-assert a BIN or MMDB wrapper

### Requirement: MMDB Lookup decodes only mapped paths
`MMDB.LookupRecord` SHALL decode only the Field map paths. Unused MMDB keys SHALL be skipped. Each path SHALL decode as the Field's type (`string` or `uint32`). The wrapper MUST NOT decode the whole row into `map[string]any`.

#### Scenario: Country map skips unused keys
- **WHEN** the Field map is `maxmind_country` and the file is a GeoIP2 Country MMDB
- **THEN** Lookup fills country and continent from the mapped paths
- **AND** unused locale name keys are not materialized

#### Scenario: uint32 ASN path
- **WHEN** the Field map includes `autonomous_system_number` with type `uint32` and Record key `asn`
- **THEN** Lookup decodes that path as uint32
- **AND** the Record ASN is the string form with an `AS` prefix when the number has none

### Requirement: BIN Lookup applies mapped Get_all columns
`BIN.LookupRecord` SHALL call `Get_all` once and SHALL copy only mapped paths onto the Record. Unused Get_all columns SHALL not be written. Path `asn` SHALL use the Get_all Asn field. Lookup MUST NOT call `Get_asn`.

#### Scenario: ASN-only map does not write country
- **WHEN** the Field map is `ip2location_asn`
- **THEN** Lookup does not set country
- **AND** Record ASN is the Get_all Asn value when the file has that column

### Requirement: Open and hot-swap are per format
The format-wrapper package SHALL expose one BIN wrapper and one MMDB wrapper. IPinfo and MaxMind files SHALL open through the MMDB wrapper. IP2Location geo and ASN files SHALL open through the BIN wrapper (one instance per catalog row). The same wrapper configuration SHALL share one open file and one download ticker. Closing the merged Lookup MUST NOT close that shared wrapper.

#### Scenario: IPinfo and MaxMind share the MMDB wrapper
- **WHEN** an enabled row has `databaseType` `mmdb`
- **THEN** that file is opened through the shared MMDB wrapper

#### Scenario: IP2Location uses one BIN wrapper per row
- **WHEN** enabled rows have `databaseType` `bin` with `fieldsPreconfigured` `ip2location_lite` and `ip2location_asn`
- **THEN** each row is opened through the shared BIN wrapper
- **AND** merge maps geo fields plus ASN onto one Record

#### Scenario: Same config shares one wrapper
- **WHEN** two plugin instances are created with the same enabled catalog rows
- **THEN** both Lookups succeed
- **AND** closing one merged Lookup does not make the other Lookup fail
