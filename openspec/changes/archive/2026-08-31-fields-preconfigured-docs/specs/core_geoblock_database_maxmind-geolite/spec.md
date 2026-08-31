## MODIFIED Requirements

### Requirement: MaxMind is an MMDB catalog row
MaxMind settings SHALL live on a `databaseSources` row with `databaseType` `mmdb` and a MaxMind Field map (`fieldsPreconfigured` `maxmind_country`, `maxmind_city`, `maxmind_asn`, `maxmind_isp`, `maxmind_domain`, or `maxmind_enterprise`, or an equivalent `fields` map). The plugin MUST pass that row to MMDB `LookupRecord` with that Field map. File location SHALL be the catalog `path`, `defaultFile`, and dated files as specified in `core_geoblock_database_url-download`. MaxMind MUST NOT use IP2Location `file=` query codes or a vendor-built permalink from `accountId:licenseKey`. Config SHALL NOT expose `maxmind_source`, `maxmind_databaseFilePath`, or `vendor`.

#### Scenario: Catalog row opens as MMDB
- **WHEN** an enabled row has `databaseType` `mmdb`, `fieldsPreconfigured` `maxmind_country`, and `path` is an existing GeoIP2-schema MMDB
- **THEN** plugin creation opens that file through the MMDB wrapper
- **AND** no IP2Location `file=` download is used

## ADDED Requirements

### Requirement: GeoIP2 ISP Domain and Enterprise presets
`maxmind_isp` SHALL map official GeoIP2 ISP binary paths: `isp` to Record `isp`, `autonomous_system_number` to `asn` with type `uint32`. `maxmind_domain` SHALL map `domain` to Record `domain`. `maxmind_enterprise` SHALL use the `maxmind_city` paths plus `traits.isp` to `isp`, `traits.domain` to `domain`, and `traits.autonomous_system_number` to `asn` with type `uint32`. Aliases `geoip2_isp`, `geoip2_domain`, and `geoip2_enterprise` SHALL resolve to those same maps. Plugin creation SHALL accept those names on an `mmdb` row.

#### Scenario: ISP preset is known
- **WHEN** an enabled row sets `databaseType` to `mmdb` and `fieldsPreconfigured` to `maxmind_isp`
- **THEN** plugin creation succeeds
- **AND** `autonomous_system_number` has type `uint32`

#### Scenario: Domain alias is known
- **WHEN** an enabled row sets `databaseType` to `mmdb` and `fieldsPreconfigured` to `geoip2_domain`
- **THEN** plugin creation succeeds

#### Scenario: Enterprise preset includes City and traits paths
- **WHEN** `fieldsPreconfigured` is `maxmind_enterprise`
- **THEN** the Field map includes `country.iso_code`, `city.names.en`, `traits.isp`, `traits.domain`, and `traits.autonomous_system_number`
