## Why

Seed file location is split across vendor-prefixed Config keys, `fileutils.Search` + `TRAEFIK_PLUGIN_GEOBLOCK_PATH`, and dated files under `databaseAutoUpdateDir`. Download already owns GET, unpack, and dated write, but not WHERE the live BIN or MMDB is. One catalog field plus one Resolve in the download component is the source of truth.

## What Changes

- **BREAKING.** Remove `ip2location_databaseFilePath`, `ip2location_asnDatabaseFilePath`, `ipinfo_databaseFilePath`, and `maxmind_databaseFilePath`.
- **BREAKING.** Remove unprefixed `databaseFilePath` and `applyDeprecatedIP2LocationSettings`. No alias copy.
- Add `path` on each `databaseDownloads` entry. A named seed requires a role pointer to that catalog key.
- The download component resolves the file to open: dated `YYYYMMDD_<key>` in `databaseAutoUpdateDir` (when dir + pointer key exist), else catalog `path` if it is a file, else `fileutils.Search` of that path or `TRAEFIK_PLUGIN_GEOBLOCK_PATH` for the vendor default filename.
- Empty pointer + empty catalog still opens the bundled default when the env (or `./name`) finds it. IP2Location ASN with empty pointer and empty path stays optional.
- Role pointers stay. HTTP GET, archive, dated-name rules unchanged. Lookup / Record unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_database_url-download`: Catalog value includes `path`. Download component owns Resolve (dated file, then `path`, then default name + env).
- `core_geoblock_database_provider`: IP2Location seed keys and `databaseFilePath` alias are gone. Pointers stay. Resolve is the shared component, not each vendor package.
- `core_geoblock_database_ipinfo-lite`: Drop `ipinfo_databaseFilePath`. Catalog `path` + pointer. Bundled `ipinfo_lite.mmdb` remains the default seed.
- `core_geoblock_database_maxmind-geolite`: Drop `maxmind_databaseFilePath`. Catalog `path` + pointer. Dummy Country MMDB remains the default seed.

## Impact

- `plugin.go` Config, `DatabaseDownload`, `catalogDownload`, `openDatabaseProvider`, `applyDeprecatedIP2LocationSettings`.
- `pkg/dbdownload` Resolve (and tests).
- `pkg/ip2location/factory.go`, `pkg/ipinfo/provider.go`, `pkg/maxmind/provider.go` seed/init path.
- README, compose examples, `plugin_config_test.go` and provider tests that set seed keys.
- `knowledge/devdocs/core_geoblock_database_provider.md`.
