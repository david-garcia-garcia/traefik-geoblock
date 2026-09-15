# Wrapper

## Language

**Wrapper**:
One open geo database file of one format (BIN or MMDB), singleton-per-config. The format owns opening a handle and looking an address up; the shared Lifecycle owns everything around that.
_Avoid_: Factory, vendor brand, slot

**Lifecycle**:
The shared open/update/dispose engine in `lifecycle.go`: resolve the file, publish a handle, log the one opened line, run keep-current, retire temp copies, and join Stop on Close. `databaseFormat` is the per-format part (open a handle, refuse a late publish, close it).
_Avoid_: resolve or hot-swap duplicated in a format; a format that logs its own open line

**Format**:
How the bytes are opened (`bin` / `mmdb`). Column maps are named presets or an operator path → Field (Record key + MMDB scalar type, default string).
_Avoid_: treating a file brand as a wrapper type; hidden vendor structs; inferring MMDB type from the Record key

**Published handle**:
The live vendor database object currently visible on a Wrapper (`*ip2loc.DB` or `*maxminddb.Reader`). Neither format locks it: a lookup may see a stale Path/Version/SourcePath or the previous generation’s handle, and the handle a swap replaced is Closed only after `readGrace`.
_Avoid_: a request-path mutex on either format; setting BIN `w.db` to nil on Close; a `closed` flag behind a mutex

## Overview

`pkg/dbwrappers` owns BIN and MMDB open/hot-swap and the named presets. `BIN.LookupRecord` and `MMDB.LookupRecord` take a `FieldMap` (path → Field) and fill a `Record`. MMDB decode uses each Field's type (`string` or `uint32`) so unused keys are skipped. The plugin never type-asserts a wrapper. Prepare expands `fieldsPreconfigured` into that map.

## How to use

- Open through `OpenBIN` or `OpenMMDB` with the plugin `New` context. Same config hash shares one file and one Updater. The wrapper logger is scoped with `key` equal to the catalog map key. BIN open from catalog uses `logging.NewOwner`: `owner_plugin` is the creating middleware; do not attach `plugin` on that logger.
- `BIN opened` / `MMDB opened` is one info line per published handle, construction and hot-swap alike. `reason` says why it is live (`init`, `seed`, `promote`, `download`), `path` is the opened file (temp copy when one exists), `source_path` is the file that handle was taken from, `version` appears when the format has a header, and `size_bytes` is that opened file’s length. When a dated catalog BIN exists and `defaultFile` is found, construction opens that seed as `reason=seed` and the promote publishes `reason=promote`. The keep-current updater copies that Latest off `New`; a later tick GETs only if the file is older than MinAge. Temp copy basename is `bin_<catalogKey>_<unixNano>.BIN`.
- Those opens go through the wrappers reclaim table (`std_go_reclaim.md`) with `bin:<catalogKey>:<hash>` / `mmdb:<catalogKey>:<hash>` keys. Sleep and Close join the keep-current loop by calling `Updater.Stop` only. Wake starts one updater again after the previous Stop has returned. Close marks the wrapper closed before that Stop, then closes the file or reader. Unreclaimed hash ends after grace. `OpenBIN` / `OpenMMDB` hand back the typed wrapper through `reclaim.OpenTyped`, then `LookupRecord(ip, fields)` via `dbprovider.Bind`.
- After Close returns, that generation stays disposed: Lookup fails, and a download that finishes afterward must not assign a live handle. BIN closes and removes a temp copy opened after Close. `db == nil` is not closed (AllowMissing may start with no file).
- One catalog row is one wrapper. A BIN ASN LITE row sets `fieldsPreconfigured: ip2location_asn` (map `asn` → `asn`). Lookup copies only that path from `Get_all`.
- `Provider.Close` must not close the shared wrapper.
- Tests call `dbwrappers.Reset`. Short-grace plugin tests call `ResetWith`.

## Gotchas

- **Do** open the Lite MMDB with `os.ReadFile` + `maxminddb.FromBytes`. After `go mod vendor`, run `scripts/apply-oschwald-yaegi-patch.ps1` so Yaegi never loads upstream mmap / `x/sys` (`incomplete type ifreq`).
- Sleep/Close on a URL-backed wrapper may wait out an in-flight GET (up to `dbutils.HTTPGetTimeout`, 30m). Reclaim parks that key only; other keys stay usable.
- Ignore a late update with a closed flag set before Stop and a re-check before publish. Do not add a mutex around BIN `LookupRecord` for that ignore.
- Neither format locks the published handle on the request path. Copy it once, then `Get_all` (BIN) or `Lookup` (MMDB) on that local. Path, Version, and SourcePath may be stale during hot-swap — accepted. After Close, a lookup fails on the `closed` atomic flag before it touches the pointer. BIN must not set `w.db` to nil (nil `Get_all` panics on `d.metaok`); MMDB may leave its pointer because a closed vendor reader fails its own lookups cleanly.
- A hot-swap Closes the handle it replaced only after `readGrace` (10s), through `disposeAfterGrace` in `lifecycle.go`, which also retires the file that handle was reading. Both formats go through it. Reclaim Close Closes the live handle.

## Pattern snippet

```go
w, err := dbwrappers.OpenMMDB(ctx, dbwrappers.MMDBConfig{Source: src}, logger)
rec, err := w.LookupRecord(ip, fields)
```

## Key files

- `pkg/dbwrappers` — BIN, MMDB, `FieldMap`, presets, `Reset`
- `pkg/dbwrappers/lifecycle.go` — shared open/update/dispose, `databaseFormat`, `readGrace`, `disposeAfterGrace`
- `pkg/dbwrappers/openreason.go` — `openReason` on the one opened line
- `vendor/.../traefik-middleware-utilities/reclaim` — wrappers table (`any`)
- `pkg/dbsource` — Resolve and Updater used by both wrappers
