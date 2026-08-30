# Source

## Language

**Source**:
The file-location and keep-current owner (`pkg/dbsource`): Resolve, GET, unpack, dated write, Updater.
_Avoid_: download, slot, dbdownload, dbmanager

**Catalog**:
Named map `databaseSources`. Each row has `vendor`, format (`databaseType`), optional `path` / `url` / `defaultFile`, and `enabled`. Operator-chosen keys plus reserved `default_ip2location`, `default_ipinfo`, `default_maxmind`, and `default_geolite`.
_Avoid_: `databaseProvider`, vendor pointers (`*_source_*`), `databaseDownloads`

**vendor**:
The field map on a catalog row: `ip2location`, `ip2location-asn`, `ipinfo`, or `maxmind`. Not the file format.
_Avoid_: using `databaseType` as the schema

**Updater**:
Keep-current loop for one source (ticker + GET).
_Avoid_: slot

## Overview

Each enabled catalog row is one wrapper plus one source. Merge happens after Lookup (`core_geoblock_database_provider`). `tools/dbdownload` is the seed CLI, not this package.

## How to use

- Put seed `path` and/or `url` on a catalog entry. Set `vendor` and matching `databaseType` (`bin` / `mmdb`).
- Lookup modes insert reserved rows when the key is absent: `default_ip2location` (enabled; free LITE ZIP + `IP2LOCATION-LITE-DB1.IPV6.BIN`), `default_ipinfo` (disabled; `ipinfo_lite.mmdb`), `default_maxmind` (disabled; dummy `GeoIP2-Country-Test.mmdb`), `default_geolite` (disabled; unofficial P3TERX Country GET). Keep an operator-defined reserved row. Do not commit a live GeoLite file.
- Omitted `enabled` means on. Zero enabled rows in a lookup mode fails `Prepare`. Unknown `vendor` or format mismatch on an enabled row fails `Prepare`. Unknown `databaseType`/`archive` fails `New`. A bound URL with empty `databaseAutoUpdateDir` WARNs and uses `os.TempDir()`/`traefik-geoblock`.
- Resolve order: newest `YYYYMMDD_<catalogKey>` in the auto-update dir, else catalog `path` if that path is an existing file (operator full path). A set `path` that is not a file WARNs `seed was specified but not found`. Else `{TRAEFIK_PLUGIN_GEOBLOCK_PATH}/seeds/<defaultFile>` then `{env}/<defaultFile>`. Empty `defaultFile` skips bundled search. No directory walk. `ip2location-asn` ships no `defaultFile`. There is no `*_databaseFilePath`.
- Wrapper and source logs include `key` (the `databaseSources` map key).
- `Start` returns a nil Updater when the URL is empty.

## Gotchas

- Failed GET/unpack errors use `DownloadHint` only. Do not log the URL (tokens may be in the query).
- `TRAEFIK_PLUGIN_GEOBLOCK_PATH` must be the plugin root. Unset → say it must be set. Set but exact files missing → those paths plus “probably not the plugin root”.
- Disable `default_ip2location` when another country row should win or be the only source.

## Pattern snippet

```go
path, err := dbsource.Resolve(cfg, logger)
u, err := dbsource.Start(cfg, logger, onUpdate)
```

## Key files

- `pkg/dbsource` — Config, Resolve, Update, unpack, Updater
- `pkg/geoblock/config.go` — `databaseSources`, `insertReservedCatalog`, `catalogSource`
- `pkg/dbutils` — HTTP GET, `DatedKeyGlob`, `DownloadHint`
