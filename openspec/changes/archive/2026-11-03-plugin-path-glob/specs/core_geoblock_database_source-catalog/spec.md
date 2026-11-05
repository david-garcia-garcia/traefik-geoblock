## MODIFIED Requirements

### Requirement: Shipped seed and download rows
When `mode` is `enrich` or `enrichandblock` and the operator did not define a reserved key, plugin creation SHALL insert:

- `default_ip2location`: enabled; `databaseType` `bin`; `archive` `zip`; official free LITE ZIP URL; `defaultFile` `IP2LOCATION-LITE-DB1.IPV6.BIN`; `fieldsPreconfigured` `ip2location_lite`
- `default_ipinfo`: disabled; `databaseType` `mmdb`; `defaultFile` `ipinfo_lite.mmdb`; `fieldsPreconfigured` `ipinfo_lite`
- `default_maxmind`: disabled; `databaseType` `mmdb`; `defaultFile` `GeoIP2-Country-Test.mmdb`; `fieldsPreconfigured` `maxmind_country`
- `default_geolite`: disabled; `databaseType` `mmdb`; `archive` `none`; unofficial P3TERX Country GET URL; `fieldsPreconfigured` `maxmind_country`

If the operator already defined that key, the plugin MUST keep that row. `defaultFile` is a basename. When no dated file and no existing catalog `path` exist, the plugin SHALL locate that basename as specified in `core_geoblock_database_url-download` (plugin directory from an exact env path, an env glob, or the unset default globs; `seeds/<defaultFile>` then `<defaultFile>` at that directory). It MUST NOT walk a directory for a basename. It MUST NOT compute a second on-disk plugin directory. An ASN LITE row (`databaseType` `bin`, `fieldsPreconfigured` `ip2location_asn`) SHALL NOT ship a `defaultFile`.

#### Scenario: IPinfo seed is a catalog row
- **WHEN** plugin creation runs and `databaseSources` has no `default_ipinfo` key
- **THEN** the catalog contains `default_ipinfo` with `databaseType` `mmdb`, `enabled` false, `defaultFile` `ipinfo_lite.mmdb`, and `fieldsPreconfigured` `ipinfo_lite`

#### Scenario: Operator reserved row is kept
- **WHEN** the operator set `databaseSources.default_ip2location` to a custom row
- **THEN** plugin creation keeps that row

#### Scenario: Bundled defaultFile uses url-download Search
- **WHEN** no dated file and no existing catalog `path` exist
- **AND** `defaultFile` is set
- **THEN** the plugin locates that file as specified in `core_geoblock_database_url-download`
- **AND** does not derive a plugin directory other than that Search
