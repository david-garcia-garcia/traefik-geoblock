# Source

## Language

**Source**:
The file-location and keep-current owner (`pkg/dbsource`): Resolve, GET, unpack, dated write, Updater.
_Avoid_: download, slot, dbdownload, dbmanager

**Catalog**:
Named map `databaseSources`. Each row has format (`databaseType`), optional `path` / `url` / `defaultFile`, a column map (`fields` or `fieldsPreconfigured`), and `enabled`. Operator-chosen keys plus reserved `default_ip2location`, `default_ipinfo`, `default_maxmind`, and `default_geolite`.
_Avoid_: `databaseProvider`, `vendor`, vendor pointers (`*_source_*`), `databaseDownloads`

**fields**:
Operator map of on-disk path → Record key (`country_short` → `country`, `country.iso_code` → `country`). A value may be that key string (MMDB type `string`) or `{key, type}` with `type` `string` or `uint32`. `maxmind_asn` sets `autonomous_system_number` to `uint32`. Do not set with `fieldsPreconfigured`.
_Avoid_: allowlist of Record keys; combining with a preset; inferring MMDB type from the Record key

**fieldsPreconfigured**:
Named vendor map (`ip2location_db8`, `ipinfo_lite`, `maxmind_country`, `maxmind_isp`, …). Prepare expands it into `fields`. Format must match `databaseType`. Operator tables: `docs/fields-preconfigured.md`.
_Avoid_: inventing a `vendor` key; combining with `fields`

**Updater**:
Keep-current loop for one source (ticker: promote Latest on disk, then GET if stale).
_Avoid_: slot

**Plugin directory**:
The on-disk Traefik plugin tree Search joins (`seeds/<defaultFile>` then `<defaultFile>`). Located from `TRAEFIK_PLUGIN_GEOBLOCK_PATH` as an exact directory, an env glob, or the unset default globs.
_Avoid_: Plugin root (the Yaegi entrypoint package); `GoPath()`; walking for a basename

**Search**:
The `fileutils` locator for a bundled file under the plugin directory.
_Avoid_: directory walk; a second plugin-directory calculation

## Overview

Each enabled catalog row is one wrapper plus one source. Merge happens after Lookup (`core_geoblock_database_lookup`). `tools/dbdownload` is the seed CLI, not this package.

## How to use

- Put seed `path` and/or `url` on a catalog entry. Set `databaseType` (`bin` / `mmdb`). Set `fieldsPreconfigured` or `fields` (path → Record key or `{key, type}`), not both. Empty both fails `Prepare` with `set fields or fieldsPreconfigured; plugin does not start; this middleware is not applied`. Unknown Record keys, field types, or preset names fail `Prepare`. Preset format must match the row. Full name list: `docs/fields-preconfigured.md`.
- Lookup modes insert reserved rows when the key is absent: `default_ip2location` (enabled; free LITE ZIP + `IP2LOCATION-LITE-DB1.IPV6.BIN`), `default_ipinfo` (disabled; `ipinfo_lite.mmdb`), `default_maxmind` (disabled; dummy `GeoIP2-Country-Test.mmdb`), `default_geolite` (disabled; unofficial P3TERX Country GET). Keep an operator-defined reserved row. Do not commit a live GeoLite file.
- Omitted `enabled` means on. Zero enabled rows in a lookup mode fails `Prepare`. Unknown or empty `databaseType` on an enabled row fails `Prepare`. Unknown `databaseType`/`archive` fails `New`. A bound URL with empty `databaseAutoUpdateDir` WARNs and uses `os.TempDir()`/`traefik-geoblock`. A bound URL with no on-disk file (including a set `defaultFile` Search cannot locate) WARNs `no database file yet; waiting for auto-update` and starts keep-current for BIN and MMDB. Missing `defaultFile` with no URL fails New.
- Resolve order: newest `YYYYMMDD_<catalogKey>` in the auto-update dir, else catalog `path` if that path is an existing file (operator full path). A set `path` that is not a file WARNs `seed was specified but not found`. Else Search locates the plugin directory and opens `{pluginDir}/seeds/<defaultFile>` then `{pluginDir}/<defaultFile>`. Exact `TRAEFIK_PLUGIN_GEOBLOCK_PATH` directory skips glob. A set env glob is exclusive (zero directory matches do not fall through). Unset env tries `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock` then `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock`. Empty `defaultFile` skips bundled search. No directory walk. Do not calculate a second plugin directory. An ASN LITE row (`databaseType: bin`, `fieldsPreconfigured: ip2location_asn`) ships no `defaultFile`; BIN open allows a missing file when both `path` and `defaultFile` are empty, or when the row has a URL. MMDB open allows a missing file the same way. There is no `*_databaseFilePath`.
- If that dated Latest file cannot be opened, initialize WARNs and opens catalog `path` then `defaultFile`. Plugin creation MUST NOT fail when a seed is readable. Keep-current may retry Latest; a failed hot-swap leaves the seed published. Do not delete the dated file.
- Need the bundled `defaultFile` without dated Latest winning: `dbsource.BundledFile` (same Search as Resolve’s last step). BIN seed-first initialize calls that.
- Wrapper and source logs include `key` (the `databaseSources` map key).
- `Start` returns a nil Updater when Dir or Key is empty (nothing to watch). URL is not required: a tick promotes Latest already on disk (this process or another writer on the share), then GETs only when a URL is set and Latest is missing or older than MinAge. `Stop` ends the ticker and waits for that goroutine to exit. An in-flight GET still calls `onUpdate` (Sleep only parks the ticker; the wrapper is still live). Close sets the wrapper disposed flag before Stop; that flag refuses the swap. Wrappers Sleep/Close call this Stop only; do not add a second join.

## Gotchas

- Failed GET/unpack errors use `DownloadHint` only. Do not log the URL (tokens may be in the query).
- `Stop` waits out an in-flight GET (`HTTPGetTimeout` is 30m). `HTTPGet` has no `context`.
- `TRAEFIK_PLUGIN_GEOBLOCK_PATH` may be an exact plugin directory or a glob. Unset tries the default globs. Env glob with zero directory matches does not fall through. File miss under a chosen directory (only remaining Resolve path) is WARN. Exclusive glob/directory miss is WARN. BundledFile miss while a dated file already won is DEBUG. Unset with no default match WARNs that the env must be set. Do not document a second plugin-directory calculation.
- Disable `default_ip2location` when another country row should win or be the only source.

## Pattern snippet

```go
path, err := dbsource.Resolve(cfg, logger)
seed, err := dbsource.BundledFile(cfg, logger)
u, err := dbsource.Start(cfg, logger, onUpdate)
```

## Key files

- `pkg/dbsource` — Config, Resolve, Update, unpack, Updater
- `pkg/fileutils/fileutils.go` — `Search`, `PluginPathEnv`
- `pkg/geoblock/config.go` — `databaseSources`, `insertReservedCatalog`, `catalogSource`
- `pkg/dbutils` — HTTP GET, `DatedKeyGlob`, `DownloadHint`
