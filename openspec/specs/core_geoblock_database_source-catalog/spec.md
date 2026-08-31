## Purpose

Defines the geo catalog as the only operator model for lookup: each `databaseSources` row names format (`databaseType`), a column map (`fields` or `fieldsPreconfigured`), bundled seed file, and whether it is enabled, so enrich can open N sources without `databaseProvider` or a `vendor` key.

## Requirements

### Requirement: Catalog rows are the lookup sources
When `mode` is `enrich` or `enrichandblock`, the plugin SHALL open every `databaseSources` row that is enabled and SHALL merge their Lookups into one Record. Config MUST NOT expose `databaseProvider`, `vendor`, `ip2location_source_geo`, `ip2location_source_asn`, `ipinfo_source`, or `maxmind_source`. An enabled row MUST set `databaseType` to `bin` or `mmdb`. Unknown or empty `databaseType` on an enabled row SHALL fail plugin creation.

#### Scenario: Empty lookup config opens default IP2Location
- **WHEN** `mode` is `enrichandblock` and the operator did not set `databaseSources`
- **THEN** plugin creation opens `default_ip2location`
- **AND** does not open `default_ipinfo`, `default_maxmind`, or `default_geolite`

#### Scenario: Unknown databaseType fails
- **WHEN** an enabled catalog row sets `databaseType` to `csv`
- **THEN** plugin creation fails

### Requirement: Enabled rows name a column map
An enabled row MUST set `fields` or `fieldsPreconfigured`, not both. Empty both SHALL fail plugin creation. The failure error MUST state that the plugin does not start and that this middleware is not applied. `fieldsPreconfigured` SHALL be a named preset whose format matches the row `databaseType`. Unknown preset names or a format mismatch SHALL fail plugin creation. After validate, the plugin SHALL expand the preset into `fields` and SHALL clear `fieldsPreconfigured` so a later Prepare is not both-set.

#### Scenario: Unknown preset fails
- **WHEN** an enabled row sets `fieldsPreconfigured` to `not-a-preset`
- **THEN** plugin creation fails

#### Scenario: Preset format mismatch fails
- **WHEN** an enabled row sets `databaseType` to `bin` and `fieldsPreconfigured` to `ipinfo_lite`
- **THEN** plugin creation fails

#### Scenario: Both maps fail
- **WHEN** an enabled row sets `fields` and `fieldsPreconfigured`
- **THEN** plugin creation fails

#### Scenario: Empty both maps names the implication
- **WHEN** an enabled row sets neither `fields` nor `fieldsPreconfigured`
- **THEN** plugin creation fails
- **AND** the error text includes `plugin does not start`
- **AND** the error text includes `this middleware is not applied`

### Requirement: Field values name a Record key and MMDB scalar type
`fields` SHALL map on-disk path to a Field: Record key plus MMDB scalar type. A YAML value MAY be the Record key string (type `string`) or `{ key, type }` with `type` `string` or `uint32`. Empty `type` SHALL be `string`. Unknown Record keys or unknown types SHALL fail plugin creation. Type SHALL NOT be inferred from the Record key. The `maxmind_asn` preset SHALL set `autonomous_system_number` to type `uint32`. IPinfo `asn` SHALL stay type `string`. BIN Lookup SHALL ignore type (columns are strings).

#### Scenario: Shorthand field is string
- **WHEN** an enabled row sets `fields.country_code` to `country`
- **THEN** plugin creation succeeds
- **AND** that path decodes as an MMDB string

#### Scenario: Object field sets uint32
- **WHEN** an enabled row sets `fields.autonomous_system_number` to `{ key: asn, type: uint32 }`
- **THEN** plugin creation succeeds
- **AND** that path decodes as an MMDB uint32

#### Scenario: Unknown field type fails
- **WHEN** an enabled row sets a field `type` to `float64`
- **THEN** plugin creation fails

### Requirement: Enabled omitted means on
A catalog row with omitted `enabled` SHALL be treated as enabled. A row with `enabled` false SHALL NOT be opened, SHALL NOT start auto-update, and SHALL NOT participate in merge. Lookup modes with zero enabled rows SHALL fail plugin creation.

#### Scenario: Operator disables the shipped default
- **WHEN** `mode` is `enrich` and `databaseSources.default_ip2location.enabled` is false
- **AND** no other row is enabled
- **THEN** plugin creation fails

#### Scenario: Operator enables IPinfo seed
- **WHEN** `mode` is `enrich` and `databaseSources.default_ipinfo.enabled` is true
- **AND** `default_ip2location.enabled` is false
- **THEN** plugin creation opens the IPinfo Lite seed
- **AND** does not open the IP2Location default

### Requirement: Shipped seed and download rows
When `mode` is `enrich` or `enrichandblock` and the operator did not define a reserved key, plugin creation SHALL insert:

- `default_ip2location`: enabled; `databaseType` `bin`; `archive` `zip`; official free LITE ZIP URL; `defaultFile` `IP2LOCATION-LITE-DB1.IPV6.BIN`; `fieldsPreconfigured` `ip2location_lite`
- `default_ipinfo`: disabled; `databaseType` `mmdb`; `defaultFile` `ipinfo_lite.mmdb`; `fieldsPreconfigured` `ipinfo_lite`
- `default_maxmind`: disabled; `databaseType` `mmdb`; `defaultFile` `GeoIP2-Country-Test.mmdb`; `fieldsPreconfigured` `maxmind_country`
- `default_geolite`: disabled; `databaseType` `mmdb`; `archive` `none`; unofficial P3TERX Country GET URL; `fieldsPreconfigured` `maxmind_country`

If the operator already defined that key, the plugin MUST keep that row. `defaultFile` is a basename. Resolve SHALL open `{TRAEFIK_PLUGIN_GEOBLOCK_PATH}/seeds/<defaultFile>` then `{env}/<defaultFile>` when no dated file and no existing catalog `path` exist. An ASN LITE row (`databaseType` `bin`, `fieldsPreconfigured` `ip2location_asn`) SHALL NOT ship a `defaultFile`.

#### Scenario: IPinfo seed is a catalog row
- **WHEN** plugin creation runs and `databaseSources` has no `default_ipinfo` key
- **THEN** the catalog contains `default_ipinfo` with `databaseType` `mmdb`, `enabled` false, `defaultFile` `ipinfo_lite.mmdb`, and `fieldsPreconfigured` `ipinfo_lite`

#### Scenario: Operator reserved row is kept
- **WHEN** the operator set `databaseSources.default_ip2location` to a custom row
- **THEN** plugin creation keeps that row

### Requirement: Merge fills empty keys only
Lookup SHALL visit enabled sources in lexicographic catalog-key order. For each meta key, the first non-empty value wins. A later source MUST NOT overwrite a non-empty value. A source that returns an error SHALL be skipped for that IP. If every enabled source errors, Lookup SHALL return an error. Column sets SHALL come from that row's Field map (preset or operator `fields`).

#### Scenario: Two sources fill different keys
- **WHEN** enabled sources are `default_ip2location` (`fieldsPreconfigured` `ip2location_lite`) and `asnlite` (`fieldsPreconfigured` `ip2location_asn`)
- **AND** geo lookup returns country `US` and ASN lookup returns `AS15169`
- **THEN** the merged Record country is `US`
- **AND** the merged Record ASN is `AS15169`

#### Scenario: First key wins country
- **WHEN** enabled sources `a_maxmind` and `b_ipinfo` both return a country
- **THEN** the merged country is the MaxMind value (`a_maxmind` sorts first)

### Requirement: Operator preset tables live in docs
The operator-facing tables of `fieldsPreconfigured` names, vendor packages, and Record keys SHALL live in `docs/fields-preconfigured.md`. That file SHALL list every shipped preset name (including aliases). `README.md` SHALL link to `docs/fields-preconfigured.md` from the `fieldsPreconfigured` section and MUST NOT embed the IP2Location package tables.

#### Scenario: README points at the docs file
- **WHEN** an operator opens `README.md` at the `fieldsPreconfigured` heading
- **THEN** that section links to `docs/fields-preconfigured.md`
- **AND** that section does not contain the IP2Location DB1–DB26 package table

#### Scenario: Docs file lists IPinfo and MaxMind presets
- **WHEN** an operator opens `docs/fields-preconfigured.md`
- **THEN** the file includes tables for `ipinfo_lite`, `ipinfo_core`, `ipinfo_plus`, `maxmind_country`, `maxmind_city`, `maxmind_asn`, `maxmind_isp`, `maxmind_domain`, and `maxmind_enterprise`
- **AND** each row names the Record keys that preset fills
