## Purpose

Defines IPinfo Lite as a selectable geo database: one MMDB for country and ASN, a committed snapshot so plugin creation works without a download token, and token-only auto-update of the official Lite MMDB URL.

## ADDED Requirements

### Requirement: IPinfo settings are vendor-prefixed
IPinfo-only settings SHALL live on Traefik Config as `ipinfo_databaseFilePath`, `ipinfo_databaseAutoUpdate`, `ipinfo_databaseAutoUpdateDir`, and `ipinfo_databaseAutoUpdateToken`. The plugin MUST pass those fields to the IPinfo provider only. IPinfo downloads MUST NOT use IP2Location `file=` query codes.

#### Scenario: Prefixed keys reach IPinfo only
- **WHEN** `databaseProvider` is `ipinfo` and `ipinfo_databaseFilePath` is set to an existing MMDB
- **THEN** plugin creation opens that file as the IPinfo Lite database
- **AND** IP2Location `file=` download settings are not used

### Requirement: Bundled Lite MMDB is the default seed
When `ipinfo_databaseFilePath` is empty, the IPinfo provider SHALL open the committed `ipinfo_lite.mmdb` snapshot (or the newest dated file in the auto-update directory when auto-update is on and that directory already has one). A stale bundled snapshot SHALL still open. Plugin creation MUST succeed without `ipinfo_databaseAutoUpdateToken` when a seed file exists.

#### Scenario: Empty path uses bundled snapshot
- **WHEN** `databaseProvider` is `ipinfo` and `ipinfo_databaseFilePath` is empty and no dated auto-update file exists
- **THEN** plugin creation opens the committed `ipinfo_lite.mmdb`

#### Scenario: Auto-update dir wins over seed
- **WHEN** `ipinfo_databaseAutoUpdate` is true and the auto-update directory contains a dated `YYYYMMDD_ipinfo_lite.mmdb`
- **THEN** plugin creation opens that dated file instead of the bundled snapshot

### Requirement: IPinfo auto-update is token-only
When `ipinfo_databaseAutoUpdate` is true, `ipinfo_databaseAutoUpdateDir` MUST be set or plugin creation SHALL fail. Download SHALL run only when `ipinfo_databaseAutoUpdateToken` is also set. The download URL SHALL be `https://ipinfo.io/data/ipinfo_lite.mmdb` with `token` as a query parameter. Stored files SHALL be named `YYYYMMDD_ipinfo_lite.mmdb`. After a successful download the provider SHALL open the new file without restarting the plugin. When auto-update is true and the token is empty, the provider MUST log an error and MUST keep the seed file; plugin creation MUST NOT fail for the missing token.

#### Scenario: Auto-update without directory fails creation
- **WHEN** `ipinfo_databaseAutoUpdate` is true and `ipinfo_databaseAutoUpdateDir` is empty
- **THEN** plugin creation fails

#### Scenario: Auto-update without token keeps seed
- **WHEN** `ipinfo_databaseAutoUpdate` is true, a directory is set, a seed MMDB exists, and `ipinfo_databaseAutoUpdateToken` is empty
- **THEN** plugin creation succeeds
- **AND** no IPinfo download is attempted
- **AND** an error is logged that the token is required

#### Scenario: Token download stores a dated MMDB
- **WHEN** auto-update is on, a directory and token are set, and a download succeeds
- **THEN** the file is stored as `YYYYMMDD_ipinfo_lite.mmdb` in that directory
- **AND** subsequent lookups use that file

### Requirement: Lite lookup maps country code and ASN fields
For a public IP found in the Lite MMDB, the provider SHALL set country used for allow/block to Lite `country_code`. It SHALL also expose country name (`country`), continent, continent code, ISP from `as_name`, domain from `as_domain`, and ASN from `asn` including any `AS` prefix present in the database. Region and city SHALL be empty. `requestHeaderEnrich` SHALL write every mapped header; an empty field SHALL be the string `null`.

#### Scenario: Known public IP
- **WHEN** the IPinfo provider looks up `8.8.8.8` in the Lite MMDB
- **THEN** country used for allow/block is `US`
- **AND** country name is `United States`
- **AND** ASN is `AS15169`

#### Scenario: Empty region and city are written as null
- **WHEN** `requestHeaderEnrich` maps headers to `region` and `city` and the lookup is IPinfo Lite
- **THEN** those headers are set to `null`
