## MODIFIED Requirements

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
