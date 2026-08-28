## Why

Three providers each built a vendor download URL from token and package code. The first apply moved only HTTP GET; lock, ticker, unpack, and dated write stayed copied in three `autoupdate.go` files. Operators also cannot name catalog entries freely: reserved `geo`/`asn` keys bind the map to one provider role. One download component plus explicit pointers replaces both the builders and the leftover copies.

## What Changes

- **BREAKING.** Remove vendor URL builders and these Config keys: `*_databaseAutoUpdate`, `*_databaseAutoUpdateDir`, `*_databaseAutoUpdateToken`, `*_databaseAutoUpdateCode`, `ip2location_asnDatabaseAutoUpdate`, `ip2location_asnDatabaseAutoUpdateCode`, and the unprefixed download aliases (`databaseAutoUpdate`, `databaseAutoUpdateToken`, `databaseAutoUpdateCode`).
- Add a named catalog `databaseDownloads`. Keys are operator-chosen. Each value is `{ url, headers, databaseType, archive }`. `databaseType` is `bin` or `mmdb`. `archive` is `none`, `zip`, or `tar.gz` (empty may default from a path extension). URL may carry `?token=`. Headers are optional.
- Providers bind with pointers: `ip2location_download_geo`, `ip2location_download_asn`, `ipinfo_download`, `maxmind_download`. Empty pointer = no download (seed/path still opens). A pointer to a missing catalog key fails `New`. Unused pointers for other providers are ignored.
- One shared dir: `databaseAutoUpdateDir` (required when any bound entry has a URL).
- One download component (GET, lock, temp, unpack-by-`archive`, date-by-`databaseType`, dated write, ticker, find-latest). The provider only opens/hot-swaps the path. Cadence is an argument (IP2Location ~30 days, IPinfo/MaxMind ~24h).
- Dated files are `YYYYMMDD_<catalogKey>` plus the extension the component writes (`.BIN` or `.mmdb`).
- **BREAKING.** Empty URL / empty pointer no longer downloads IP2Location LITE for free. Operators paste the official lite ZIP URL and set `archive: zip`, `databaseType: bin`.
- **BREAKING.** Withdraw `file=` token-download behavior. Operators put `file=` on the URL themselves.
- Seed file paths stay vendor-prefixed. Unprefixed `databaseFilePath` alias is unchanged.
- Lookup / Record schema is unchanged.

## Capabilities

### New Capabilities

- `core_geoblock_database_url-download`: Catalog, pointers, `databaseType`, `archive`, shared download component, no vendor URL builder.

### Modified Capabilities

- `core_geoblock_database_provider`: Download GET/unpack/ticker is the shared component. Vendor packages keep Lookup open and hot-swap. IP2Location download keys leave this spec; pointers stay vendor-prefixed.
- `core_geoblock_database_ipinfo-lite`: Drop token/code/official URL download. `ipinfo_download` pointer + shared dir. Keep seed path, bundled Lite MMDB, and lookup.
- `core_geoblock_database_maxmind-geolite`: Drop permalink + `accountId:licenseKey` builder and edition code. `maxmind_download` pointer + shared dir. Keep dummy seed and GeoIP2 lookup. No ASN pointer.
- `core_geoblock_database_token-download-file`: Withdraw. Token `file=` is no longer built by the plugin.

## Impact

- `plugin.go` Config, `CreateConfig`, `applyLegacyDatabaseKeys` (download aliases removed; `databaseAutoUpdateDir` is the shared dir).
- New download component (likely `pkg/dbdownload`) using `pkg/dbutils` GET/hint/date helpers. `pkg/ip2location/autoupdate.go`, `pkg/ipinfo/autoupdate.go`, `pkg/maxmind/autoupdate.go` lose the update loop.
- README, `docker-compose.yml`, Pester token/lite auto-update labels.
- `knowledge/devdocs/core_geoblock_database_provider.md` usage (implement updates it).
- Existing auto-update dirs named `YYYYMMDD_geo` / old vendor names must be re-fetched under catalog keys.
