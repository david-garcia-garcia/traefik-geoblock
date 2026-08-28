## Purpose

Defines the central HTTP download of geo database files from operator-supplied catalog entries, so vendors no longer build download URLs and no longer copy the update loop.

## Requirements

### Requirement: Downloads are a named catalog
Traefik Config SHALL expose `databaseDownloads` as a map of operator-chosen name to `{ url, headers, databaseType, archive }`. `headers` is an optional map of HTTP header name to value. The URL MAY include query parameters (including a download token). Catalog keys are not reserved (`geo` and `asn` are allowed as ordinary names). `CreateConfig` SHALL initialize the map.

#### Scenario: Geo-named catalog key is an ordinary name
- **WHEN** `databaseDownloads` contains a key `geo`
- **THEN** plugin creation does not fail because of that key name

#### Scenario: Query token is accepted
- **WHEN** a catalog entry `url` is an https URL that includes a `token` query parameter
- **THEN** plugin creation does not fail because of that query parameter
- **AND** the plugin does not rewrite the URL

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
The plugin SHALL expose `ip2location_download_geo`, `ip2location_download_asn`, `ipinfo_download`, and `maxmind_download` as optional catalog key names. An empty pointer SHALL NOT download. A non-empty pointer that is not a key in `databaseDownloads` SHALL fail plugin creation. Pointers that do not apply to the selected `databaseProvider` SHALL be ignored.

#### Scenario: Missing catalog key fails creation
- **WHEN** `databaseProvider` is `ipinfo` and `ipinfo_download` is `lite` and `databaseDownloads` has no `lite` key
- **THEN** plugin creation fails

#### Scenario: Empty pointer keeps the seed
- **WHEN** all download pointers are empty and a seed database file exists
- **THEN** plugin creation succeeds
- **AND** no HTTP download is attempted

#### Scenario: IP2Location ASN pointer
- **WHEN** `databaseProvider` is `ip2location` and `ip2location_download_asn` names a catalog entry with a URL and `databaseAutoUpdateDir` is set
- **THEN** plugin creation succeeds
- **AND** that entry is eligible for download as the ASN file

### Requirement: Shared directory is required when a bound URL is set
When any pointer that applies to the selected provider names a catalog entry with a non-empty `url`, `databaseAutoUpdateDir` MUST be set or plugin creation SHALL fail. A shared download component SHALL GET the URL (follow redirects, apply headers), unpack by `archive`, read the file date by `databaseType`, and store `YYYYMMDD_<catalogKey>` plus `.BIN` or `.mmdb`. After a successful download the provider SHALL open the new file without restarting the plugin. The component MUST NOT log the URL. Failed responses SHALL be reported with `DownloadHint` only.

#### Scenario: Bound URL without directory fails creation
- **WHEN** `ipinfo_download` names an entry with a URL and `databaseAutoUpdateDir` is empty
- **THEN** plugin creation fails

#### Scenario: Successful download is dated by catalog key
- **WHEN** catalog key `litezip` downloads successfully
- **THEN** the stored file name starts with `YYYYMMDD_litezip`

### Requirement: The plugin does not build vendor download URLs
The plugin MUST NOT construct IP2Location `file=` URLs, IPinfo `ipinfo_{code}.mmdb?token=` URLs, or MaxMind permalink URLs from a token or package code.

#### Scenario: No token or code keys
- **WHEN** Config is decoded
- **THEN** there is no `databaseAutoUpdateToken` or `databaseAutoUpdateCode` (prefixed or unprefixed) that builds a download URL
