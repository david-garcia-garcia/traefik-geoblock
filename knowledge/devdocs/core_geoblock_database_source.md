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
Keep-current loop for one source (ticker + GET).
_Avoid_: slot

## Overview

Each enabled catalog row is one wrapper plus one source. Merge happens after Lookup (`core_geoblock_database_lookup`). `tools/dbdownload` is the seed CLI, not this package.

## How to use

- Put seed `path` and/or `url` on a catalog entry. Set `databaseType` (`bin` / `mmdb`). Set `fieldsPreconfigured` or `fields` (path → Record key or `{key, type}`), not both. Empty both fails `Prepare` with `set fields or fieldsPreconfigured; plugin does not start; this middleware is not applied`. Unknown Record keys, field types, or preset names fail `Prepare`. Preset format must match the row. Full name list: `docs/fields-preconfigured.md`.
- Lookup modes insert reserved rows when the key is absent: `default_ip2location` (enabled; free LITE ZIP + `IP2LOCATION-LITE-DB1.IPV6.BIN`), `default_ipinfo` (disabled; `ipinfo_lite.mmdb`), `default_maxmind` (disabled; dummy `GeoIP2-Country-Test.mmdb`), `default_geolite` (disabled; unofficial P3TERX Country GET). Keep an operator-defined reserved row. Do not commit a live GeoLite file.
- Omitted `enabled` means on. Zero enabled rows in a lookup mode fails `Prepare`. Unknown or empty `databaseType` on an enabled row fails `Prepare`. Unknown `databaseType`/`archive` fails `New`. A bound URL with empty `databaseAutoUpdateDir` WARNs and uses `os.TempDir()`/`traefik-geoblock`.
- Resolve order: newest `YYYYMMDD_<catalogKey>` in the auto-update dir, else catalog `path` if that path is an existing file (operator full path). A set `path` that is not a file WARNs `seed was specified but not found`. Else `{TRAEFIK_PLUGIN_GEOBLOCK_PATH}/seeds/<defaultFile>` then `{env}/<defaultFile>`. Empty `defaultFile` skips bundled search. No directory walk. An ASN LITE row (`databaseType: bin`, `fieldsPreconfigured: ip2location_asn`) ships no `defaultFile`; BIN open allows a missing file when both `path` and `defaultFile` are empty. There is no `*_databaseFilePath`.
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
