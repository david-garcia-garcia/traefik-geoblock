# Wrapper

## Language

**Wrapper**:
One open geo database file of one format (BIN or MMDB): resolve, open, hot-swap, singleton-per-config.
_Avoid_: Factory, vendor brand, slot

**Format**:
How the bytes are opened (`bin` / `mmdb`). Column maps are named presets or an operator path → Field (Record key + MMDB scalar type, default string).
_Avoid_: treating a file brand as a wrapper type; hidden vendor structs; inferring MMDB type from the Record key

## Overview

`pkg/dbwrappers` owns BIN and MMDB open/hot-swap and the named presets. `BIN.LookupRecord` and `MMDB.LookupRecord` take a `FieldMap` (path → Field) and fill a `Record`. MMDB decode uses each Field's type (`string` or `uint32`) so unused keys are skipped. The plugin never type-asserts a wrapper. Prepare expands `fieldsPreconfigured` into that map.

## How to use

- Open through `OpenBIN` or `OpenMMDB` with the plugin `New` context. Same config hash shares one file and one Updater. The wrapper logger is scoped with `key` equal to the catalog map key. BIN open from catalog uses `logging.NewOwner`: `owner_plugin` is the creating middleware; do not attach `plugin` on that logger.
- `BIN initialized` / `BIN hot-swapped`: `path` / `new_path` is the opened file (temp copy when one exists). `source_path` is the dated catalog or seed file. Temp copy basename is `bin_<catalogKey>_<unixNano>.BIN`.
- Those opens go through `reclaim.Open` (`std_go_reclaim.md`) with `bin:<catalogKey>:<hash>` / `mmdb:<catalogKey>:<hash>` keys. Create watches the incarnation lifetime and calls `close` when it is canceled. Unreclaimed hash ends after grace. The caller asserts `*BIN` / `*MMDB`, then `LookupRecord(ip, fields)` via `dbprovider.Bind`.
- One catalog row is one wrapper. A BIN ASN LITE row sets `fieldsPreconfigured: ip2location_asn` (map `asn` → `asn`). Lookup copies only that path from `Get_all`.
- `Provider.Close` must not close the shared wrapper.
- Tests call `dbwrappers.Reset`. Short-grace plugin tests call `ResetWith`.

## Gotchas

- **Do** open the Lite MMDB with `os.ReadFile` + `maxminddb.FromBytes`. After `go mod vendor`, run `scripts/apply-oschwald-yaegi-patch.ps1` so Yaegi never loads upstream mmap / `x/sys` (`incomplete type ifreq`).

## Pattern snippet

```go
w, err := dbwrappers.OpenMMDB(ctx, dbwrappers.MMDBConfig{Source: src}, logger)
rec, err := w.LookupRecord(ip, fields)
```

## Key files

- `pkg/dbwrappers` — BIN, MMDB, `FieldMap`, presets, `Reset`
- `pkg/reclaim` — process table (`any`)
- `pkg/dbsource` — Resolve and Updater used by both wrappers
