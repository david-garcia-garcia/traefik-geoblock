## Purpose

Defines the central HTTP download of geo database files from operator-supplied catalog entries, so vendors no longer build download URLs and no longer copy the update loop.

## Requirements

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

### Requirement: databaseType and archive describe the file
`databaseType` SHALL be `bin` or `mmdb`. `archive` SHALL be `none`, `zip`, or `tar.gz`. Unknown values SHALL fail plugin creation. Empty `archive` MAY be inferred from the URL path extension (`.zip`, `.tar.gz`, `.tgz`, `.mmdb`, `.bin`/`.BIN`). If `archive` is empty and the path has no recognized extension, plugin creation or the download SHALL fail without sniffing the body. The plugin MUST NOT log the URL.

#### Scenario: Unknown databaseType fails creation
- **WHEN** a catalog entry sets `databaseType` to `csv`
- **THEN** plugin creation fails

#### Scenario: Unknown archive fails creation
- **WHEN** a catalog entry sets `archive` to `rar`
- **THEN** plugin creation fails

#### Scenario: Token URL without archive fails unless set
- **WHEN** a catalog `url` path has no file extension and `archive` is empty
- **THEN** plugin creation or the download fails
- **AND** the error MUST NOT include the URL

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

### Requirement: Pointer databaseType matches the provider
A bound pointer of the selected provider SHALL fail plugin creation when the named catalog row has a non-empty `databaseType` that is not the format that provider opens. IP2Location SHALL accept only `bin`. IPinfo and MaxMind SHALL accept only `mmdb`. An empty `databaseType` on the row MAY take the provider format. Unknown `databaseType` or `archive` on any catalog row SHALL still fail plugin creation.

#### Scenario: IP2Location pointed at mmdb fails creation
- **WHEN** `databaseProvider` is `ip2location` and `ip2location_source_geo` names a catalog row whose `databaseType` is `mmdb`
- **THEN** plugin creation fails

#### Scenario: IPinfo pointed at bin fails creation
- **WHEN** `databaseProvider` is `ipinfo` and `ipinfo_source` names a catalog row whose `databaseType` is `bin`
- **THEN** plugin creation fails

### Requirement: The plugin does not build vendor download URLs
The plugin MUST NOT construct IP2Location `file=` URLs, IPinfo `ipinfo_{code}.mmdb?token=` URLs, or MaxMind permalink URLs from a token or package code.

#### Scenario: No token or code keys
- **WHEN** Config is decoded
- **THEN** there is no `databaseAutoUpdateToken` or `databaseAutoUpdateCode` (prefixed or unprefixed) that builds a download URL

### Requirement: Download component resolves the file location
The shared source component SHALL choose the file the provider opens. When a pointer and `databaseAutoUpdateDir` are set, the newest dated `YYYYMMDD_<catalogKey>` file in that directory SHALL win. Else if the bound catalog `path` is an existing file, that file SHALL be used. `path` is an operator file path (typically absolute), not a basename and not a directory to walk. Else the component SHALL search `TRAEFIK_PLUGIN_GEOBLOCK_PATH` for the vendor default filename (including under `seeds/`). Empty `path` and empty `url` SHALL still search that env for the bundled file. Config SHALL NOT expose `ip2location_databaseFilePath`, `ip2location_asnDatabaseFilePath`, `ipinfo_databaseFilePath`, `maxmind_databaseFilePath`, or unprefixed `databaseFilePath`.

#### Scenario: Dated catalog file wins
- **WHEN** a pointer names catalog key `lite` and `databaseAutoUpdateDir` contains a dated `YYYYMMDD_lite` file
- **THEN** plugin creation opens that dated file

#### Scenario: Empty path uses bundled default
- **WHEN** source pointers are empty and `TRAEFIK_PLUGIN_GEOBLOCK_PATH` contains the vendor default filename
- **THEN** plugin creation opens that bundled file

#### Scenario: No seed Config keys
- **WHEN** Config is decoded
- **THEN** there is no `ip2location_databaseFilePath`, `ipinfo_databaseFilePath`, `maxmind_databaseFilePath`, or `databaseFilePath`
