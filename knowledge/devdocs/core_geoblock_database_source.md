# Source

## Language

**Source**:
The file-location and keep-current owner (`pkg/dbsource`): Resolve, GET, unpack, dated write, Updater.
_Avoid_: download, slot, dbdownload, dbmanager

**Catalog**:
Named map `databaseSources` of seed path and/or URL. Operator-chosen keys plus reserved `default_ip2location` and `default_geolite`.
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
- `New` inserts reserved `default_ip2location` (free IP2Location geo LITE ZIP, `bin`/`zip`) and `default_geolite` (unofficial P3TERX Country MMDB, `mmdb`/`none`) unless the operator already set that key. Empty `ip2location_source_geo` binds `default_ip2location`. Empty `maxmind_source` binds `default_geolite`. Keep an operator-defined reserved row. Do not commit a live GeoLite file; Resolve still opens the official dummy Country seed until a dated file exists.
- A pointer to a missing key WARNs and is treated as empty (IP2Location geo → `default_ip2location`; MaxMind → `default_geolite`; IPinfo → bundled seed; ASN → no ASN). Unknown `databaseType`/`archive` fails `New`. A bound pointer whose `databaseType` does not match the provider (`bin` vs `mmdb`) fails `New`. A bound URL with empty `databaseAutoUpdateDir` WARNs and uses `os.TempDir()`/`traefik-geoblock`. Unused pointers for another provider are ignored.
- Resolve order: newest `YYYYMMDD_<catalogKey>` in the auto-update dir, else catalog `path` if that path is an existing file (operator full path). A set `path` that is not a file WARNs `seed was specified but not found`. Else `{TRAEFIK_PLUGIN_GEOBLOCK_PATH}/seeds/<DefaultFileName>` then `{env}/<DefaultFileName>`. No directory walk. ASN has no `DefaultFileName`. There is no `*_databaseFilePath`.
- Wrapper and source logs include `key` (the `databaseSources` map key).
- `Start` returns a nil Updater when the URL is empty.

## Gotchas

- Failed GET/unpack errors use `DownloadHint` only. Do not log the URL (tokens may be in the query).
- `TRAEFIK_PLUGIN_GEOBLOCK_PATH` must be the plugin root. Unset → say it must be set. Set but exact files missing → those paths plus “probably not the plugin root”.

## Pattern snippet

```go
path, err := dbsource.Resolve(cfg, logger)
u, err := dbsource.Start(cfg, logger, onUpdate)
```

## Key files

- `pkg/dbsource` — Config, Resolve, Update, unpack, Updater
- `pkg/geoblock/plugin.go` — `databaseSources`, pointers, `catalogSource`
- `pkg/dbutils` — HTTP GET, `DatedKeyGlob`, `DownloadHint`
