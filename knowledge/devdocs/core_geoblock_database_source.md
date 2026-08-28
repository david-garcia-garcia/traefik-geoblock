# Source

## Language

**Source**:
The file-location and keep-current owner (`pkg/dbsource`): Resolve, GET, unpack, dated write, Updater.
_Avoid_: download, slot, dbdownload, dbmanager

**Catalog**:
Named map `databaseSources` of seed path and/or URL.
_Avoid_: reserved `geo`/`asn` keys, `databaseDownloads`

**Pointer**:
Vendor-prefixed Config key that names a catalog entry for one role.
_Avoid_: download pointer, `*_download_*`

**Updater**:
Keep-current loop for one source (ticker + GET).
_Avoid_: slot

## Overview

GEO and ASN are two independent wrapper + source pairs. The provider holds the wrappers. `tools/dbdownload` is the seed CLI, not this package.

## How to use

- Put seed `path` and/or `url` on a catalog entry. Bind with `ip2location_source_geo`, `ip2location_source_asn`, `ipinfo_source`, or `maxmind_source`.
- Empty pointer = bundled default / `TRAEFIK_PLUGIN_GEOBLOCK_PATH`. A named `path` needs a pointer. A pointer to a missing key, unknown `databaseType`/`archive`, or a bound URL without `databaseAutoUpdateDir` fails `New`. Unused pointers for another provider are ignored.
- Resolve order: newest `YYYYMMDD_<catalogKey>` in the auto-update dir, else catalog `path` if that path is an existing file (operator full path), else the vendor default filename under `seeds/`. There is no `*_databaseFilePath`.
- `Start` returns a nil Updater when the URL is empty.

## Gotchas

- Failed GET/unpack errors use `DownloadHint` only. Do not log the URL (tokens may be in the query).

## Pattern snippet

```go
path, err := dbsource.Resolve(cfg, logger)
u, err := dbsource.Start(cfg, logger, onUpdate)
```

## Key files

- `pkg/dbsource` — Config, Resolve, Update, unpack, Updater
- `plugin.go` — `databaseSources`, pointers, `catalogSource`
- `pkg/dbutils` — HTTP GET, `DatedKeyGlob`, `DownloadHint`
