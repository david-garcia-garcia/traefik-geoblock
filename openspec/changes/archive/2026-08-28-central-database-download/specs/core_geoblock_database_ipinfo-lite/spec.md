## MODIFIED Requirements

### Requirement: IPinfo settings are vendor-prefixed
IPinfo-only settings SHALL live on Traefik Config as `ipinfo_databaseFilePath` and `ipinfo_download`. The plugin MUST pass those fields to the IPinfo provider only. HTTP download SHALL use the catalog entry named by `ipinfo_download` and `databaseAutoUpdateDir` as specified in `core_geoblock_database_url-download`. IPinfo MUST NOT use IP2Location `file=` query codes or a vendor-built `ipinfo_{code}.mmdb?token=` URL.

#### Scenario: Prefixed keys reach IPinfo only
- **WHEN** `databaseProvider` is `ipinfo` and `ipinfo_databaseFilePath` is set to an existing MMDB
- **THEN** plugin creation opens that file as the IPinfo database
- **AND** no IP2Location `file=` download is used

### Requirement: Bundled Lite MMDB is the default seed
When `ipinfo_databaseFilePath` is empty, the IPinfo provider SHALL open the committed `ipinfo_lite.mmdb` snapshot (or the newest dated file for the `ipinfo_download` catalog key in `databaseAutoUpdateDir` when that directory already has one). A stale bundled snapshot SHALL still open. Plugin creation MUST succeed without a download pointer when a seed file exists.

#### Scenario: Empty path uses bundled snapshot
- **WHEN** `databaseProvider` is `ipinfo` and `ipinfo_databaseFilePath` is empty and no dated catalog file exists
- **THEN** plugin creation opens the committed `ipinfo_lite.mmdb`

#### Scenario: Auto-update dir wins over seed
- **WHEN** `ipinfo_download` is `lite` and `databaseAutoUpdateDir` contains a dated `YYYYMMDD_lite` MMDB
- **THEN** plugin creation opens that dated file instead of the bundled snapshot

## REMOVED Requirements

### Requirement: IPinfo auto-update is token-only
**Reason**: Vendor token/code URL builders are replaced by `databaseDownloads` plus `ipinfo_download`.
**Migration**: Add a catalog entry (`url` = `https://ipinfo.io/data/ipinfo_{lite|core|plus}.mmdb?token=…`, `databaseType` = `mmdb`, `archive` = `none`), set `ipinfo_download` to that key, and set `databaseAutoUpdateDir`.
