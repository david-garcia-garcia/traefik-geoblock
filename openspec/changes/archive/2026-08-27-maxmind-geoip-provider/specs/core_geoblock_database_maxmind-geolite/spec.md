## Purpose

Defines MaxMind GeoLite2 / GeoIP2 as a selectable geo database: GeoIP2 nested country lookup, a committed official dummy Country MMDB so plugin creation works without a download token, and official permalink auto-update authenticated with `accountId:licenseKey`.

## ADDED Requirements

### Requirement: MaxMind settings are vendor-prefixed
MaxMind-only settings SHALL live on Traefik Config as `maxmind_databaseFilePath`, `maxmind_databaseAutoUpdate`, `maxmind_databaseAutoUpdateDir`, `maxmind_databaseAutoUpdateToken`, and `maxmind_databaseAutoUpdateCode`. The plugin MUST pass those fields to the MaxMind provider only. MaxMind downloads MUST NOT use IP2Location `file=` query codes. Default `maxmind_databaseAutoUpdateCode` SHALL be `GeoLite2-Country`. Allowed codes SHALL be country or city editions (`GeoLite2-Country`, `GeoLite2-City`, `GeoIP2-Country`, `GeoIP2-City`). An ASN-only code SHALL fail plugin creation.

#### Scenario: Prefixed keys reach MaxMind only
- **WHEN** `databaseProvider` is `maxmind` and `maxmind_databaseFilePath` is set to an existing GeoIP2-schema MMDB
- **THEN** plugin creation opens that file as the MaxMind database
- **AND** IP2Location `file=` download settings are not used

#### Scenario: ASN edition is rejected
- **WHEN** `maxmind_databaseAutoUpdateCode` is `GeoLite2-ASN`
- **THEN** plugin creation fails

### Requirement: Dummy Country MMDB is the default seed
When `maxmind_databaseFilePath` is empty, the MaxMind provider SHALL open the committed `GeoIP2-Country-Test.mmdb` snapshot (or the newest dated file in the auto-update directory when auto-update is on and that directory already has one). Plugin creation MUST succeed without `maxmind_databaseAutoUpdateToken` when a seed file exists. The committed file SHALL be MaxMind’s official dummy Country fixture, not a live GeoLite download.

#### Scenario: Empty path uses bundled dummy
- **WHEN** `databaseProvider` is `maxmind` and `maxmind_databaseFilePath` is empty and no dated auto-update file exists
- **THEN** plugin creation opens the committed `GeoIP2-Country-Test.mmdb`

#### Scenario: Auto-update dir wins over seed
- **WHEN** `maxmind_databaseAutoUpdate` is true and the auto-update directory contains a dated `YYYYMMDD_*.mmdb` for the configured edition
- **THEN** plugin creation opens that dated file instead of the bundled dummy

### Requirement: MaxMind auto-update is official permalink + token
When `maxmind_databaseAutoUpdate` is true, `maxmind_databaseAutoUpdateDir` MUST be set or plugin creation SHALL fail. Download SHALL run only when `maxmind_databaseAutoUpdateToken` parses as `accountId:licenseKey` (HTTP Basic Auth username and password). The download URL SHALL be `https://download.maxmind.com/geoip/databases/{EDITION_ID}/download?suffix=tar.gz` with `EDITION_ID` equal to `maxmind_databaseAutoUpdateCode`. The client MUST follow redirects. Stored files SHALL be named `YYYYMMDD_<edition>.mmdb`. After a successful download the provider SHALL open the new file without restarting the plugin. When auto-update is true and the token is empty or not `accountId:licenseKey`, the provider MUST log an error and MUST keep the seed file; plugin creation MUST NOT fail for the missing token when a seed exists.

#### Scenario: Auto-update without directory fails creation
- **WHEN** `maxmind_databaseAutoUpdate` is true and `maxmind_databaseAutoUpdateDir` is empty
- **THEN** plugin creation fails

#### Scenario: Auto-update without token keeps seed
- **WHEN** `maxmind_databaseAutoUpdate` is true, a directory is set, a seed MMDB exists, and `maxmind_databaseAutoUpdateToken` is empty
- **THEN** plugin creation succeeds
- **AND** no MaxMind download is attempted
- **AND** an error is logged that the token is required

#### Scenario: Token that is not accountId:licenseKey keeps seed
- **WHEN** auto-update is on, a directory is set, a seed exists, and the token has no `:` separator
- **THEN** plugin creation succeeds
- **AND** no MaxMind download is attempted
- **AND** an error is logged

### Requirement: GeoIP2 lookup maps nested country.iso_code
For a public IP found in a GeoIP2 / GeoLite2 Country or City MMDB, the provider SHALL set country used for allow/block to nested `country.iso_code`. It SHALL NOT use IPinfo `country_code` tags. It SHALL use located `country`, not `registered_country`. Country name SHALL come from `country.names` English when present. Continent and city/region SHALL be filled when the file has those maps. ASN and ISP SHALL stay empty on a Country or City file. `requestHeaderEnrich` SHALL write every mapped header; an empty field SHALL be the string `null`.

#### Scenario: Dummy Country test IP
- **WHEN** the MaxMind provider looks up a documented dummy-fixture IP in `GeoIP2-Country-Test.mmdb`
- **THEN** country used for allow/block is the ISO code stored under `country.iso_code`

#### Scenario: Empty enrich fields are written as null
- **WHEN** `requestHeaderEnrich` maps a header to `asn` and the open file is GeoLite2-Country or the dummy Country fixture
- **THEN** that header is set to `null`
