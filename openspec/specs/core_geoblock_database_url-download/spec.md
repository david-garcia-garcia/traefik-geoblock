## Purpose

Defines the central HTTP download of geo database files from operator-supplied catalog entries, so vendors no longer build download URLs and no longer copy the update loop.

## Requirements

### Requirement: Downloads are a named catalog
Traefik Config SHALL expose `databaseSources` as a map of operator-chosen name to `{ url, headers, databaseType, archive, path, defaultFile, fields, fieldsPreconfigured, enabled }`. `headers` is an optional map of HTTP header name to value. `path` is an optional existing file used as the seed when no dated catalog file exists. It is an operator file path (typically absolute), not a basename and not a directory to walk. `defaultFile` is an optional basename resolved under `TRAEFIK_PLUGIN_GEOBLOCK_PATH` as specified in this spec. The URL MAY include query parameters (including a download token). Catalog keys `geo` and `asn` are ordinary names. Reserved keys `default_ip2location`, `default_ipinfo`, `default_maxmind`, and `default_geolite` SHALL be inserted when the operator did not define them, as specified in `core_geoblock_database_source-catalog`. If the operator already defined a reserved key, the plugin MUST keep that row. `CreateConfig` SHALL initialize the map. Config MUST NOT expose `vendor`.

#### Scenario: Geo-named catalog key is an ordinary name
- **WHEN** `databaseSources` contains a key `geo`
- **THEN** plugin creation does not fail because of that key name

#### Scenario: Query token is accepted
- **WHEN** a catalog entry `url` is an https URL that includes a `token` query parameter
- **THEN** plugin creation does not fail because of that query parameter
- **AND** the plugin does not rewrite the URL

#### Scenario: Catalog path is a seed
- **WHEN** an enabled row with `databaseType` `mmdb` has `path` set to an existing MMDB and that entry has no URL
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

### Requirement: Shared directory is required when a bound URL is set
When any **enabled** catalog row has a non-empty `url` and `databaseAutoUpdateDir` is empty, the plugin SHALL WARN and SHALL use `filepath.Join(os.TempDir(), "traefik-geoblock")` as the directory. A shared source component SHALL GET the URL (follow redirects, apply headers), unpack by `archive`, read the file date by `databaseType`, and store `YYYYMMDD_<catalogKey>` plus `.BIN` or `.mmdb`. After a successful download the source SHALL open the new file without restarting the plugin. The component MUST NOT log the URL. Failed responses SHALL be reported with `DownloadHint` only. Until a dated file exists, Resolve SHALL still open `{pluginDir}/seeds/<defaultFile>` when `defaultFile` is set and that file exists, with `pluginDir` located as specified in `Download component resolves the file location`.

#### Scenario: Bound URL without directory uses temp dir
- **WHEN** an enabled row has a URL and `databaseAutoUpdateDir` is empty
- **THEN** plugin creation succeeds
- **AND** a WARN says the auto-update dir is missing
- **AND** dated files are written under the process temp dir in a `traefik-geoblock` folder

#### Scenario: Successful download is dated by catalog key
- **WHEN** catalog key `litezip` downloads successfully
- **THEN** the stored file name starts with `YYYYMMDD_litezip`

### Requirement: Preset format matches databaseType
An enabled row SHALL fail plugin creation when `fieldsPreconfigured` names a preset whose format is not the row `databaseType`. An enabled row SHALL fail plugin creation when `databaseType` is empty or unknown. Unknown `archive` on any catalog row SHALL still fail plugin creation.

#### Scenario: IPinfo preset on bin fails creation
- **WHEN** an enabled row has `databaseType` `bin` and `fieldsPreconfigured` `ipinfo_lite`
- **THEN** plugin creation fails

#### Scenario: MaxMind preset on bin fails creation
- **WHEN** an enabled row has `databaseType` `bin` and `fieldsPreconfigured` `maxmind_country`
- **THEN** plugin creation fails

### Requirement: The plugin does not build vendor download URLs
The plugin MUST NOT construct IP2Location `file=` URLs, IPinfo `ipinfo_{code}.mmdb?token=` URLs, or MaxMind permalink URLs from a token or package code.

#### Scenario: No token or code keys
- **WHEN** Config is decoded
- **THEN** there is no `databaseAutoUpdateToken` or `databaseAutoUpdateCode` (prefixed or unprefixed) that builds a download URL

### Requirement: Download component resolves the file location
The shared source component SHALL choose the file the source opens. When `databaseAutoUpdateDir` is set, the newest dated `YYYYMMDD_<catalogKey>` file in that directory SHALL win. Else if the catalog `path` is an existing file, that file SHALL be used. `path` is an operator file path (typically absolute), not a basename and not a directory to walk. When `path` is non-empty and is not an existing file, the plugin SHALL WARN `seed was specified but not found` and SHALL include that path, then SHALL continue Resolve. Else when `defaultFile` is set, the plugin SHALL open the first existing file of `{pluginDir}/seeds/<defaultFile>` then `{pluginDir}/<defaultFile>`. It MUST NOT walk a directory for a basename. It MUST NOT compute a second on-disk plugin directory (Traefik’s plugin Manager already owns that path). `pluginDir` SHALL be located from `TRAEFIK_PLUGIN_GEOBLOCK_PATH` as follows:

- When the env is set and names an existing directory, that directory SHALL be used and glob matching SHALL be skipped.
- When the env is set and is not an existing directory, the env value SHALL be treated as a glob. Matching directories SHALL be considered in lexicographic order. Zero directory matches MUST NOT fall through to the unset defaults.
- When the env is unset, the plugin SHALL try `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock` then `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock`.

When several directories match a glob, the plugin SHALL keep the first lexicographic match that contains `seeds/<defaultFile>` for that call, else the first that contains `<defaultFile>` at that directory. None SHALL be the same not-found outcome as a missing bundled file today. When the env is unset and those defaults locate no directory or no file, the plugin SHALL log that `TRAEFIK_PLUGIN_GEOBLOCK_PATH` must be set to the plugin directory. When the env is set and both exact files are missing under the chosen directory, the plugin SHALL log those exact paths and SHALL say the env is probably not the plugin directory. A bundled-file miss that is the only remaining resolution path SHALL WARN. A bundled-file miss when a dated catalog file has already won SHALL DEBUG. Those misses MUST NOT be logged as Error. A row with `fieldsPreconfigured` `ip2location_asn` SHALL NOT set a bundled `defaultFile`. Empty ASN `path` and no dated ASN file SHALL wait for auto-update without searching `IP2LOCATION-LITE-ASN.IPV6.BIN`. Wrapper and source log lines for an open or missing file SHALL include `key` equal to the catalog map key. Config SHALL NOT expose `ip2location_databaseFilePath`, `ip2location_asnDatabaseFilePath`, `ipinfo_databaseFilePath`, `maxmind_databaseFilePath`, or unprefixed `databaseFilePath`.

#### Scenario: Dated catalog file wins
- **WHEN** an enabled row key is `lite` and `databaseAutoUpdateDir` contains a dated `YYYYMMDD_lite` file
- **THEN** plugin creation opens that dated file

#### Scenario: Empty path uses bundled default at seeds
- **WHEN** a source has no dated file and no catalog `path` and `defaultFile` is set
- **AND** `TRAEFIK_PLUGIN_GEOBLOCK_PATH` is an existing plugin directory
- **THEN** plugin creation opens `{dir}/seeds/<defaultFile>` when that file exists

#### Scenario: Missing catalog path warns
- **WHEN** a bound catalog `path` is set and that path is not an existing file
- **THEN** the plugin WARNs `seed was specified but not found` and includes that path
- **AND** Resolve continues (bundled default or empty)

#### Scenario: ASN has no bundled default search
- **WHEN** an enabled row has `fieldsPreconfigured` `ip2location_asn`, catalog `path` is empty, and no dated ASN file exists
- **THEN** plugin creation succeeds
- **AND** the plugin does not search for `IP2LOCATION-LITE-ASN.IPV6.BIN`
- **AND** a log says no database file yet and includes `key` for that catalog entry

#### Scenario: Exact directory skips glob
- **WHEN** `TRAEFIK_PLUGIN_GEOBLOCK_PATH` is an existing directory that contains `seeds/<defaultFile>`
- **AND** another directory would match a glob of that env value
- **THEN** the plugin opens the file under the exact directory
- **AND** does not search the other match

#### Scenario: Env glob locates catalog unpack
- **WHEN** `TRAEFIK_PLUGIN_GEOBLOCK_PATH` is `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock`
- **AND** that glob matches one directory that contains `seeds/<defaultFile>`
- **THEN** plugin creation opens that `seeds/<defaultFile>`

#### Scenario: Env glob with zero matches does not fall through
- **WHEN** `TRAEFIK_PLUGIN_GEOBLOCK_PATH` is a glob that matches no directory
- **AND** a default unset path would have contained `seeds/<defaultFile>`
- **THEN** the plugin does not open that default path
- **AND** bundled resolution fails

#### Scenario: Unset env tries default globs
- **WHEN** a bundled default is needed and `TRAEFIK_PLUGIN_GEOBLOCK_PATH` is unset
- **AND** `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock` matches a directory that contains `seeds/<defaultFile>`
- **THEN** plugin creation opens that `seeds/<defaultFile>`

#### Scenario: Unset env with no default match names the plugin directory
- **WHEN** a bundled default is needed and `TRAEFIK_PLUGIN_GEOBLOCK_PATH` is unset
- **AND** neither default path locates the file
- **THEN** the plugin logs that the env must be set to the plugin directory

#### Scenario: Several glob matches keep the seeded directory
- **WHEN** the env glob matches two directories in lexicographic order `a` then `b`
- **AND** only `b` contains `seeds/<defaultFile>`
- **THEN** the plugin opens `{b}/seeds/<defaultFile>`

#### Scenario: Env set but exact files missing
- **WHEN** a bundled default is needed and `TRAEFIK_PLUGIN_GEOBLOCK_PATH` is set to an existing directory
- **AND** neither `{dir}/seeds/<defaultFile>` nor `{dir}/<defaultFile>` exists
- **THEN** the plugin WARNs those exact paths
- **AND** says the env is probably not the plugin directory

#### Scenario: Bundled miss while dated file won is debug
- **WHEN** `databaseAutoUpdateDir` contains a dated file for the catalog key that Resolve already chose
- **AND** `defaultFile` is set and is not present under the plugin directory
- **THEN** the bundled-file miss is logged at DEBUG
- **AND** plugin creation still opens the dated file

#### Scenario: Wrapper logs include catalog key
- **WHEN** a BIN or MMDB wrapper initializes, waits for auto-update, or hot-swaps
- **THEN** the log line includes `key` equal to the `databaseSources` map key

#### Scenario: No seed Config keys
- **WHEN** Config is decoded
- **THEN** there is no `ip2location_databaseFilePath`, `ipinfo_databaseFilePath`, `maxmind_databaseFilePath`, or `databaseFilePath`

### Requirement: URL with no on-disk file waits for auto-update
When an enabled row has a non-empty `url` and Resolve finds no file (no dated catalog file, no existing catalog `path`, and `defaultFile` missing or empty), plugin creation SHALL WARN and SHALL start keep-current. BIN and MMDB SHALL share this wait. MMDB open SHALL allow a missing file the same way BIN does. A set `defaultFile` that cannot be located SHALL still wait when the row has a URL. A missing `defaultFile` with no URL SHALL fail plugin creation. Nil-handle lookups SHALL return an error. `banIfError` default SHALL stay true (fail-closed until a handle exists). When `banIfError` is false, a lookup error or empty country MUST NOT ban; CIDR blocks, private, country lists, and `defaultAllow` SHALL still apply. That is fail-open on lookup only, not a policy bypass.

#### Scenario: BIN URL and missing defaultFile starts
- **WHEN** an enabled BIN row has a URL and `defaultFile` set
- **AND** no dated file, catalog `path`, or bundled `defaultFile` exists
- **THEN** plugin creation succeeds
- **AND** a WARN says there is no database file yet
- **AND** keep-current starts

#### Scenario: MMDB URL and missing defaultFile starts
- **WHEN** an enabled MMDB row has a URL and `defaultFile` set
- **AND** no dated file, catalog `path`, or bundled `defaultFile` exists
- **THEN** plugin creation succeeds
- **AND** a WARN says there is no database file yet
- **AND** keep-current starts

#### Scenario: Missing defaultFile with no URL fails
- **WHEN** an enabled row has `defaultFile` set, no URL, and that file cannot be located
- **THEN** plugin creation fails

#### Scenario: banIfError false is fail-open on lookup only
- **WHEN** the wrapper has no handle yet and a lookup returns an error
- **AND** `banIfError` is false
- **THEN** the plugin does not ban for that lookup failure
- **AND** CIDR, private, country lists, and `defaultAllow` still apply

#### Scenario: banIfError true blocks a lookup error
- **WHEN** the wrapper has no handle yet and a lookup returns an error
- **AND** `banIfError` is true
- **THEN** the plugin bans with `block:error`
