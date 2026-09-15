# Wrapper

## Language

**Wrapper**:
One open geo database file of one format (BIN or MMDB): resolve, open, hot-swap, singleton-per-config.
_Avoid_: Factory, vendor brand, slot

**Format**:
How the bytes are opened (`bin` / `mmdb`). Column maps are named presets or an operator path → Field (Record key + MMDB scalar type, default string).
_Avoid_: treating a file brand as a wrapper type; hidden vendor structs; inferring MMDB type from the Record key

**Published handle**:
The live vendor database object currently visible on a Wrapper (`*ip2loc.DB` or `*maxminddb.Reader`). MMDB publishes under a write lock. BIN does not: a lookup may see a stale Path/Version/SourcePath or the previous generation’s handle.
_Avoid_: a request-path mutex on BIN Get_all; setting BIN `w.db` to nil on Close; a lock helper shared by BIN and MMDB

## Overview

`pkg/dbwrappers` owns BIN and MMDB open/hot-swap and the named presets. `BIN.LookupRecord` and `MMDB.LookupRecord` take a `FieldMap` (path → Field) and fill a `Record`. MMDB decode uses each Field's type (`string` or `uint32`) so unused keys are skipped. The plugin never type-asserts a wrapper. Prepare expands `fieldsPreconfigured` into that map.

## How to use

- Open through `OpenBIN` or `OpenMMDB` with the plugin `New` context. Same config hash shares one file and one Updater. The wrapper logger is scoped with `key` equal to the catalog map key. BIN open from catalog uses `logging.NewOwner`: `owner_plugin` is the creating middleware; do not attach `plugin` on that logger.
- `BIN initialized` / `BIN hot-swapped`: `path` / `new_path` is the opened file (temp copy when one exists). `source_path` is the file that handle was taken from. `size_bytes` is that opened file’s length. When a dated catalog BIN exists and `defaultFile` is found, initialize opens that seed first (`pending_source_path` is the dated file). The keep-current updater copies that Latest off `New` and hot-swaps; a later tick GETs only if the file is older than MinAge. Temp copy basename is `bin_<catalogKey>_<unixNano>.BIN`.
- Those opens go through the wrappers reclaim table (`std_go_reclaim.md`) with `bin:<catalogKey>:<hash>` / `mmdb:<catalogKey>:<hash>` keys. Sleep and Close join the keep-current loop by calling `Updater.Stop` only. Wake starts one updater again after the previous Stop has returned. Close marks the wrapper closed before that Stop, then closes the file or reader. Unreclaimed hash ends after grace. The caller asserts `*BIN` / `*MMDB`, then `LookupRecord(ip, fields)` via `dbprovider.Bind`.
- After Close returns, that generation stays disposed: Lookup fails, and a download that finishes afterward must not assign a live handle. BIN closes and removes a temp copy opened after Close. `db == nil` is not closed (AllowMissing may start with no file).
- One catalog row is one wrapper. A BIN ASN LITE row sets `fieldsPreconfigured: ip2location_asn` (map `asn` → `asn`). Lookup copies only that path from `Get_all`.
- `Provider.Close` must not close the shared wrapper.
- Tests call `dbwrappers.Reset`. Short-grace plugin tests call `ResetWith`.

## Gotchas

- **Do** open the Lite MMDB with `os.ReadFile` + `maxminddb.FromBytes`. After `go mod vendor`, run `scripts/apply-oschwald-yaegi-patch.ps1` so Yaegi never loads upstream mmap / `x/sys` (`incomplete type ifreq`).
- Sleep/Close on a URL-backed wrapper may wait out an in-flight GET (up to `dbutils.HTTPGetTimeout`, 30m). Reclaim parks that key only; other keys stay usable.
- Ignore a late update with a closed flag set before Stop and a re-check before publish. Do not add a mutex around BIN `LookupRecord` for that ignore.
- BIN does not lock the published handle on the request path. Copy `w.db` once, then `Get_all` on that local. Path, Version, and SourcePath may be stale during hot-swap — accepted. Close the vendor file; do not set `w.db` to nil (nil `Get_all` panics on `d.metaok`). After Close, Lookup fails on the disposed flag. MMDB still uses `sync.RWMutex` / `swapReader` / `RLock` across `db.Lookup`. `defer` Unlock on MMDB: Yaegi recovers panics without exiting.
- BIN hot-swap delayed-Closes the previous vendor handle after 10s. Reclaim Close Closes the live handle and leaves the pointer. Do not extract a helper shared with MMDB.

## Pattern snippet

```go
w, err := dbwrappers.OpenMMDB(ctx, dbwrappers.MMDBConfig{Source: src}, logger)
rec, err := w.LookupRecord(ip, fields)
```

## Key files

- `pkg/dbwrappers` — BIN, MMDB, `FieldMap`, presets, `Reset`
- `vendor/.../traefik-middleware-utilities/reclaim` — wrappers table (`any`)
- `pkg/dbsource` — Resolve and Updater used by both wrappers
