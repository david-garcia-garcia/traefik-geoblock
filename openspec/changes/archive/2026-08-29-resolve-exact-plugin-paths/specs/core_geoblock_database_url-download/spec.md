## MODIFIED Requirements

### Requirement: Download component resolves the file location
The shared source component SHALL choose the file the provider opens. When a pointer and `databaseAutoUpdateDir` are set, the newest dated `YYYYMMDD_<catalogKey>` file in that directory SHALL win. Else if the bound catalog `path` is an existing file, that file SHALL be used. `path` is an operator file path (typically absolute), not a basename and not a directory to walk. When `path` is non-empty and is not an existing file, the plugin SHALL WARN `seed was specified but not found` and SHALL include that path, then SHALL continue Resolve. Else when a vendor `DefaultFileName` is set, the plugin SHALL open the first existing file of `{TRAEFIK_PLUGIN_GEOBLOCK_PATH}/seeds/<DefaultFileName>` then `{TRAEFIK_PLUGIN_GEOBLOCK_PATH}/<DefaultFileName>`. It MUST NOT walk the env directory for a basename. When the env is unset, the plugin SHALL log that `TRAEFIK_PLUGIN_GEOBLOCK_PATH` must be set to the plugin root. When the env is set and both exact files are missing, the plugin SHALL log those exact paths and SHALL say the env is probably not the plugin root. IP2Location ASN SHALL NOT set a bundled `DefaultFileName`. Empty ASN `path` and no dated ASN file SHALL wait for auto-update without searching `IP2LOCATION-LITE-ASN.IPV6.BIN`. Wrapper and source log lines for an open or missing file SHALL include `key` equal to the catalog map key. Config SHALL NOT expose `ip2location_databaseFilePath`, `ip2location_asnDatabaseFilePath`, `ipinfo_databaseFilePath`, `maxmind_databaseFilePath`, or unprefixed `databaseFilePath`.

#### Scenario: Dated catalog file wins
- **WHEN** a pointer names catalog key `lite` and `databaseAutoUpdateDir` contains a dated `YYYYMMDD_lite` file
- **THEN** plugin creation opens that dated file

#### Scenario: Empty path uses bundled default at seeds
- **WHEN** a geo or MMDB source has no dated file and no catalog `path`
- **AND** `TRAEFIK_PLUGIN_GEOBLOCK_PATH` is the plugin root
- **THEN** plugin creation opens `{env}/seeds/<vendor default filename>` when that file exists

#### Scenario: Missing catalog path warns
- **WHEN** a bound catalog `path` is set and that path is not an existing file
- **THEN** the plugin WARNs `seed was specified but not found` and includes that path
- **AND** Resolve continues (bundled default or empty)

#### Scenario: ASN has no bundled default search
- **WHEN** `ip2location_source_asn` is bound, catalog `path` is empty, and no dated ASN file exists
- **THEN** plugin creation succeeds
- **AND** the plugin does not search for `IP2LOCATION-LITE-ASN.IPV6.BIN`
- **AND** a log says no database file yet and includes `key` for that catalog entry

#### Scenario: Env unset names the plugin root
- **WHEN** a bundled default is needed and `TRAEFIK_PLUGIN_GEOBLOCK_PATH` is unset
- **THEN** the plugin logs that the env must be set to the plugin root

#### Scenario: Env set but exact files missing
- **WHEN** a bundled default is needed and `TRAEFIK_PLUGIN_GEOBLOCK_PATH` is set
- **AND** neither `{env}/seeds/<DefaultFileName>` nor `{env}/<DefaultFileName>` exists
- **THEN** the plugin logs those exact paths
- **AND** says the env is probably not the plugin root

#### Scenario: Wrapper logs include catalog key
- **WHEN** a BIN or MMDB wrapper initializes, waits for auto-update, or hot-swaps
- **THEN** the log line includes `key` equal to the `databaseSources` map key

#### Scenario: No seed Config keys
- **WHEN** Config is decoded
- **THEN** there is no `ip2location_databaseFilePath`, `ipinfo_databaseFilePath`, `maxmind_databaseFilePath`, or `databaseFilePath`

### Requirement: Shared directory is required when a bound URL is set
When any pointer that applies to the selected provider names a catalog entry with a non-empty `url` and `databaseAutoUpdateDir` is empty, the plugin SHALL WARN and SHALL use `filepath.Join(os.TempDir(), "traefik-geoblock")` as the directory. A shared source component SHALL GET the URL (follow redirects, apply headers), unpack by `archive`, read the file date by `databaseType`, and store `YYYYMMDD_<catalogKey>` plus `.BIN` or `.mmdb`. After a successful download the provider SHALL open the new file without restarting the plugin. The component MUST NOT log the URL. Failed responses SHALL be reported with `DownloadHint` only. Until a dated file exists, Resolve SHALL still open the bundled vendor default filename when that file exists and a `DefaultFileName` is set.

#### Scenario: Bound URL without directory uses temp dir
- **WHEN** `ipinfo_source` names an entry with a URL and `databaseAutoUpdateDir` is empty
- **THEN** plugin creation succeeds
- **AND** a WARN says the auto-update dir is missing
- **AND** dated files are written under the process temp dir in a `traefik-geoblock` folder

#### Scenario: Successful download is dated by catalog key
- **WHEN** catalog key `litezip` downloads successfully
- **THEN** the stored file name starts with `YYYYMMDD_litezip`
