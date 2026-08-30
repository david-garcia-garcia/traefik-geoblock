## MODIFIED Requirements

### Requirement: Downloads are a named catalog
Traefik Config SHALL expose `databaseSources` as a map of operator-chosen name to `{ url, headers, databaseType, archive, path, vendor, defaultFile, enabled }`. `headers` is an optional map of HTTP header name to value. `path` is an optional existing file used as the seed when no dated catalog file exists. It is an operator file path (typically absolute), not a basename and not a directory to walk. `defaultFile` is an optional basename resolved under `TRAEFIK_PLUGIN_GEOBLOCK_PATH` as specified in this spec. The URL MAY include query parameters (including a download token). Catalog keys `geo` and `asn` are ordinary names. Reserved keys `default_ip2location`, `default_ipinfo`, `default_maxmind`, and `default_geolite` SHALL be inserted when the operator did not define them, as specified in `core_geoblock_database_source-catalog`. If the operator already defined a reserved key, the plugin MUST keep that row. `CreateConfig` SHALL initialize the map.

#### Scenario: Geo-named catalog key is an ordinary name
- **WHEN** `databaseSources` contains a key `geo`
- **THEN** plugin creation does not fail because of that key name

#### Scenario: Query token is accepted
- **WHEN** a catalog entry `url` is an https URL that includes a `token` query parameter
- **THEN** plugin creation does not fail because of that query parameter
- **AND** the plugin does not rewrite the URL

#### Scenario: Catalog path is a seed
- **WHEN** an enabled row with `vendor` `ipinfo` has `path` set to an existing MMDB and that entry has no URL
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

### Requirement: Shared directory is required when a bound URL is set
When any **enabled** catalog row has a non-empty `url` and `databaseAutoUpdateDir` is empty, the plugin SHALL WARN and SHALL use `filepath.Join(os.TempDir(), "traefik-geoblock")` as the directory. A shared source component SHALL GET the URL (follow redirects, apply headers), unpack by `archive`, read the file date by `databaseType`, and store `YYYYMMDD_<catalogKey>` plus `.BIN` or `.mmdb`. After a successful download the source SHALL open the new file without restarting the plugin. The component MUST NOT log the URL. Failed responses SHALL be reported with `DownloadHint` only. Until a dated file exists, Resolve SHALL still open `{env}/seeds/<defaultFile>` when `defaultFile` is set and that file exists.

#### Scenario: Bound URL without directory uses temp dir
- **WHEN** an enabled row has a URL and `databaseAutoUpdateDir` is empty
- **THEN** plugin creation succeeds
- **AND** a WARN says the auto-update dir is missing
- **AND** dated files are written under the process temp dir in a `traefik-geoblock` folder

#### Scenario: Successful download is dated by catalog key
- **WHEN** catalog key `litezip` downloads successfully
- **THEN** the stored file name starts with `YYYYMMDD_litezip`

### Requirement: Pointer databaseType matches the provider
An enabled row SHALL fail plugin creation when `databaseType` is set and does not match `vendor` (`ip2location` / `ip2location-asn` → `bin`; `ipinfo` / `maxmind` → `mmdb`). An empty `databaseType` on the row MAY take the vendor format. Unknown `databaseType` or `archive` on any catalog row SHALL still fail plugin creation.

#### Scenario: IP2Location row with mmdb fails creation
- **WHEN** an enabled row has `vendor` `ip2location` and `databaseType` `mmdb`
- **THEN** plugin creation fails

#### Scenario: IPinfo row with bin fails creation
- **WHEN** an enabled row has `vendor` `ipinfo` and `databaseType` `bin`
- **THEN** plugin creation fails

### Requirement: Download component resolves the file location
The shared source component SHALL choose the file the source opens. When `databaseAutoUpdateDir` is set, the newest dated `YYYYMMDD_<catalogKey>` file in that directory SHALL win. Else if the catalog `path` is an existing file, that file SHALL be used. `path` is an operator file path (typically absolute), not a basename and not a directory to walk. When `path` is non-empty and is not an existing file, the plugin SHALL WARN `seed was specified but not found` and SHALL include that path, then SHALL continue Resolve. Else when `defaultFile` is set, the plugin SHALL open the first existing file of `{TRAEFIK_PLUGIN_GEOBLOCK_PATH}/seeds/<defaultFile>` then `{TRAEFIK_PLUGIN_GEOBLOCK_PATH}/<defaultFile>`. It MUST NOT walk the env directory for a basename. When the env is unset, the plugin SHALL log that `TRAEFIK_PLUGIN_GEOBLOCK_PATH` must be set to the plugin root. When the env is set and both exact files are missing, the plugin SHALL log those exact paths and SHALL say the env is probably not the plugin root. `vendor` `ip2location-asn` SHALL NOT set a bundled `defaultFile`. Empty ASN `path` and no dated ASN file SHALL wait for auto-update without searching `IP2LOCATION-LITE-ASN.IPV6.BIN`. Wrapper and source log lines for an open or missing file SHALL include `key` equal to the catalog map key. Config SHALL NOT expose `ip2location_databaseFilePath`, `ip2location_asnDatabaseFilePath`, `ipinfo_databaseFilePath`, `maxmind_databaseFilePath`, or unprefixed `databaseFilePath`.

#### Scenario: Dated catalog file wins
- **WHEN** an enabled row key is `lite` and `databaseAutoUpdateDir` contains a dated `YYYYMMDD_lite` file
- **THEN** plugin creation opens that dated file

#### Scenario: Empty path uses bundled default at seeds
- **WHEN** a source has no dated file and no catalog `path` and `defaultFile` is set
- **AND** `TRAEFIK_PLUGIN_GEOBLOCK_PATH` is the plugin root
- **THEN** plugin creation opens `{env}/seeds/<defaultFile>` when that file exists

#### Scenario: Missing catalog path warns
- **WHEN** a bound catalog `path` is set and that path is not an existing file
- **THEN** the plugin WARNs `seed was specified but not found` and includes that path
- **AND** Resolve continues (bundled default or empty)

#### Scenario: ASN has no bundled default search
- **WHEN** an enabled row has `vendor` `ip2location-asn`, catalog `path` is empty, and no dated ASN file exists
- **THEN** plugin creation succeeds
- **AND** the plugin does not search for `IP2LOCATION-LITE-ASN.IPV6.BIN`
- **AND** a log says no database file yet and includes `key` for that catalog entry

#### Scenario: Env unset names the plugin root
- **WHEN** a bundled default is needed and `TRAEFIK_PLUGIN_GEOBLOCK_PATH` is unset
- **THEN** the plugin logs that the env must be set to the plugin root

#### Scenario: Env set but exact files missing
- **WHEN** a bundled default is needed and `TRAEFIK_PLUGIN_GEOBLOCK_PATH` is set
- **AND** neither `{env}/seeds/<defaultFile>` nor `{env}/<defaultFile>` exists
- **THEN** the plugin logs those exact paths
- **AND** says the env is probably not the plugin root

#### Scenario: Wrapper logs include catalog key
- **WHEN** a BIN or MMDB wrapper initializes, waits for auto-update, or hot-swaps
- **THEN** the log line includes `key` equal to the `databaseSources` map key

#### Scenario: No seed Config keys
- **WHEN** Config is decoded
- **THEN** there is no `ip2location_databaseFilePath`, `ipinfo_databaseFilePath`, `maxmind_databaseFilePath`, or `databaseFilePath`

## REMOVED Requirements

### Requirement: Providers bind with pointers
