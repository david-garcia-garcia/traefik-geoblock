## MODIFIED Requirements

### Requirement: Downloads are a named catalog
Traefik Config SHALL expose `databaseSources` as a map of operator-chosen name to `{ url, headers, databaseType, archive, path }`. `headers` is an optional map of HTTP header name to value. `path` is an optional existing file used as the seed when no dated catalog file exists. It is an operator file path (typically absolute), not a basename and not a directory to walk. The URL MAY include query parameters (including a download token). Catalog keys `geo` and `asn` are ordinary names. The key `default_ip2location` is reserved: plugin creation SHALL insert that row when the operator did not define it. The inserted row SHALL use the official free IP2Location geo LITE ZIP (`https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP`), `databaseType` `bin`, and `archive` `zip`. It SHALL NOT include an ASN source. The key `default_geolite` is reserved: plugin creation SHALL insert that row when the operator did not define it. The inserted row SHALL use the unofficial P3TERX Country MMDB (`https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb`), `databaseType` `mmdb`, and `archive` `none`. It SHALL NOT include City or ASN. The plugin MUST NOT commit a live GeoLite file. If the operator already defined `default_ip2location` or `default_geolite`, the plugin MUST keep that row. `CreateConfig` SHALL initialize the map.

#### Scenario: Geo-named catalog key is an ordinary name
- **WHEN** `databaseSources` contains a key `geo`
- **THEN** plugin creation does not fail because of that key name

#### Scenario: Query token is accepted
- **WHEN** a catalog entry `url` is an https URL that includes a `token` query parameter
- **THEN** plugin creation does not fail because of that query parameter
- **AND** the plugin does not rewrite the URL

#### Scenario: Catalog path is a seed
- **WHEN** `ipinfo_source` is `lite` and `databaseSources.lite.path` is set to an existing MMDB and that entry has no URL
- **THEN** plugin creation opens that file
- **AND** no HTTP download is attempted

#### Scenario: Default catalog row is inserted
- **WHEN** plugin creation runs and `databaseSources` has no `default_ip2location` key
- **THEN** the catalog contains `default_ip2location` with the free IP2Location geo LITE ZIP URL, `databaseType` `bin`, and `archive` `zip`

#### Scenario: Operator default catalog row is kept
- **WHEN** the operator set `databaseSources.default_ip2location` to a custom row
- **THEN** plugin creation keeps that row
- **AND** does not replace its URL or type

#### Scenario: Default geolite catalog row is inserted
- **WHEN** plugin creation runs and `databaseSources` has no `default_geolite` key
- **THEN** the catalog contains `default_geolite` with the unofficial P3TERX Country MMDB URL, `databaseType` `mmdb`, and `archive` `none`

#### Scenario: Operator default geolite catalog row is kept
- **WHEN** the operator set `databaseSources.default_geolite` to a custom row
- **THEN** plugin creation keeps that row
- **AND** does not replace its URL or type

### Requirement: Providers bind with pointers
The plugin SHALL expose `ip2location_source_geo`, `ip2location_source_asn`, `ipinfo_source`, and `maxmind_source` as optional catalog key names. Empty `ip2location_source_geo` SHALL bind `default_ip2location`. Empty `maxmind_source` SHALL bind `default_geolite`. A non-empty pointer that is not a key in `databaseSources` SHALL log a WARN naming the pointer and key, SHALL NOT fail plugin creation, and SHALL be treated as empty (IP2Location geo → `default_ip2location`; MaxMind → `default_geolite`; IPinfo → bundled seed; IP2Location ASN → no ASN). Pointers that do not apply to the selected `databaseProvider` SHALL be ignored. A named catalog `path` SHALL be used only when a pointer names that entry. An invalid `databaseProvider` SHALL fail plugin creation.

#### Scenario: Missing catalog key warns and falls back
- **WHEN** `databaseProvider` is `ipinfo` and `ipinfo_source` is `lite` and `databaseSources` has no `lite` key
- **THEN** plugin creation succeeds
- **AND** a WARN names the missing key
- **AND** the IPinfo provider opens the bundled default MMDB when that file exists

#### Scenario: Empty IP2Location geo pointer binds the default catalog
- **WHEN** `databaseProvider` is `ip2location` and `ip2location_source_geo` is empty
- **THEN** plugin creation binds `default_ip2location`
- **AND** that entry is eligible for download as the geo file

#### Scenario: Empty MaxMind pointer binds the default geolite catalog
- **WHEN** `databaseProvider` is `maxmind` and `maxmind_source` is empty
- **THEN** plugin creation binds `default_geolite`
- **AND** that entry is eligible for download as the Country file

#### Scenario: Empty pointer keeps the default seed
- **WHEN** all source pointers are empty and a bundled default database file exists
- **THEN** plugin creation succeeds

#### Scenario: IP2Location ASN pointer
- **WHEN** `databaseProvider` is `ip2location` and `ip2location_source_asn` names a catalog entry with a URL and `databaseAutoUpdateDir` is set
- **THEN** plugin creation succeeds
- **AND** that entry is eligible for download as the ASN file

#### Scenario: Missing ASN pointer warns and skips ASN
- **WHEN** `databaseProvider` is `ip2location` and `ip2location_source_asn` is `asnlite` and `databaseSources` has no `asnlite` key
- **THEN** plugin creation succeeds
- **AND** a WARN names the missing key
- **AND** no ASN file is opened

### Requirement: Shared directory is required when a bound URL is set
When any pointer that applies to the selected provider names a catalog entry with a non-empty `url` and `databaseAutoUpdateDir` is empty, the plugin SHALL WARN and SHALL use `filepath.Join(os.TempDir(), "traefik-geoblock")` as the directory. A shared source component SHALL GET the URL (follow redirects, apply headers), unpack by `archive`, read the file date by `databaseType`, and store `YYYYMMDD_<catalogKey>` plus `.BIN` or `.mmdb`. After a successful download the provider SHALL open the new file without restarting the plugin. The component MUST NOT log the URL. Failed responses SHALL be reported with `DownloadHint` only. Until a dated file exists, Resolve SHALL still open the bundled vendor default filename when that file exists.

#### Scenario: Bound URL without directory uses temp dir
- **WHEN** `ipinfo_source` names an entry with a URL and `databaseAutoUpdateDir` is empty
- **THEN** plugin creation succeeds
- **AND** a WARN says the auto-update dir is missing
- **AND** dated files are written under the process temp dir in a `traefik-geoblock` folder

#### Scenario: Successful download is dated by catalog key
- **WHEN** catalog key `litezip` downloads successfully
- **THEN** the stored file name starts with `YYYYMMDD_litezip`

## ADDED Requirements

### Requirement: Pointer databaseType matches the provider
A bound pointer of the selected provider SHALL fail plugin creation when the named catalog row has a non-empty `databaseType` that is not the format that provider opens. IP2Location SHALL accept only `bin`. IPinfo and MaxMind SHALL accept only `mmdb`. An empty `databaseType` on the row MAY take the provider format. Unknown `databaseType` or `archive` on any catalog row SHALL still fail plugin creation.

#### Scenario: IP2Location pointed at mmdb fails creation
- **WHEN** `databaseProvider` is `ip2location` and `ip2location_source_geo` names a catalog row whose `databaseType` is `mmdb`
- **THEN** plugin creation fails

#### Scenario: IPinfo pointed at bin fails creation
- **WHEN** `databaseProvider` is `ipinfo` and `ipinfo_source` names a catalog row whose `databaseType` is `bin`
- **THEN** plugin creation fails
