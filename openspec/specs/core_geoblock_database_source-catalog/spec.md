## Purpose

Defines the geo catalog as the only operator model for lookup: each `databaseSources` row names vendor schema, bundled seed file, and whether it is enabled, so enrich can open N sources (including different vendors) without `databaseProvider` or vendor pointers.

## Requirements

### Requirement: Catalog rows are the lookup sources
When `mode` is `enrich` or `enrichandblock`, the plugin SHALL open every `databaseSources` row that is enabled and SHALL merge their Lookups into one Record. Config MUST NOT expose `databaseProvider`, `ip2location_source_geo`, `ip2location_source_asn`, `ipinfo_source`, or `maxmind_source`. An enabled row MUST set `vendor` to `ip2location`, `ip2location-asn`, `ipinfo`, or `maxmind`. Unknown `vendor` SHALL fail plugin creation. Empty `vendor` on an enabled row SHALL fail plugin creation. `databaseType` SHALL stay `bin` or `mmdb` (format). `vendor` `ip2location` and `ip2location-asn` REQUIRE `bin`. `vendor` `ipinfo` and `maxmind` REQUIRE `mmdb`. A mismatch SHALL fail plugin creation.

#### Scenario: Empty lookup config opens default IP2Location
- **WHEN** `mode` is `enrichandblock` and the operator did not set `databaseSources`
- **THEN** plugin creation opens `default_ip2location`
- **AND** does not open `default_ipinfo`, `default_maxmind`, or `default_geolite`

#### Scenario: Unknown vendor fails
- **WHEN** an enabled catalog row sets `vendor` to `csv`
- **THEN** plugin creation fails

#### Scenario: Vendor format mismatch fails
- **WHEN** an enabled row sets `vendor` to `ipinfo` and `databaseType` to `bin`
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

- `default_ip2location`: enabled; `vendor` `ip2location`; `databaseType` `bin`; `archive` `zip`; official free LITE ZIP URL; `defaultFile` `IP2LOCATION-LITE-DB1.IPV6.BIN`
- `default_ipinfo`: disabled; `vendor` `ipinfo`; `databaseType` `mmdb`; `defaultFile` `ipinfo_lite.mmdb`
- `default_maxmind`: disabled; `vendor` `maxmind`; `databaseType` `mmdb`; `defaultFile` `GeoIP2-Country-Test.mmdb`
- `default_geolite`: disabled; `vendor` `maxmind`; `databaseType` `mmdb`; `archive` `none`; unofficial P3TERX Country GET URL

If the operator already defined that key, the plugin MUST keep that row. `defaultFile` is a basename. Resolve SHALL open `{TRAEFIK_PLUGIN_GEOBLOCK_PATH}/seeds/<defaultFile>` then `{env}/<defaultFile>` when no dated file and no existing catalog `path` exist. `ip2location-asn` SHALL NOT ship a `defaultFile`.

#### Scenario: IPinfo seed is a catalog row
- **WHEN** plugin creation runs and `databaseSources` has no `default_ipinfo` key
- **THEN** the catalog contains `default_ipinfo` with `vendor` `ipinfo`, `enabled` false, and `defaultFile` `ipinfo_lite.mmdb`

#### Scenario: Operator reserved row is kept
- **WHEN** the operator set `databaseSources.default_ip2location` to a custom row
- **THEN** plugin creation keeps that row

### Requirement: Merge fills empty keys only
Lookup SHALL visit enabled sources in lexicographic catalog-key order. For each meta key, the first non-empty value wins. A later source MUST NOT overwrite a non-empty value. A source that returns an error SHALL be skipped for that IP. If every enabled source errors, Lookup SHALL return an error. Column sets SHALL come from the `vendor` code table (`ip2location` geo fields; `ip2location-asn` ASN only; `ipinfo` Lite/Core tags; `maxmind` nested `country.iso_code`). Config MUST NOT list columns as YAML.

#### Scenario: Two vendors fill different keys
- **WHEN** enabled sources are `default_ip2location` (`vendor` `ip2location`) and `asnlite` (`vendor` `ip2location-asn`)
- **AND** geo lookup returns country `US` and ASN lookup returns `AS15169`
- **THEN** the merged Record country is `US`
- **AND** the merged Record ASN is `AS15169`

#### Scenario: First key wins country
- **WHEN** enabled sources `a_maxmind` and `b_ipinfo` both return a country
- **THEN** the merged country is the MaxMind value (`a_maxmind` sorts first)
