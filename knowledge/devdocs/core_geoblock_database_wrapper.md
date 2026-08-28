# Wrapper

## Language

**Wrapper**:
One open geo database file of one format (BIN or MMDB): resolve, open, hot-swap, singleton-per-config.
_Avoid_: Factory, vendor brand, slot

**Format**:
How the bytes are opened (`bin` / `mmdb`). IPinfo and MaxMind share MMDB; they differ only in field tags.
_Avoid_: treating vendor as the share axis

## Overview

`pkg/dbwrappers` owns BIN and MMDB. A vendor provider holds one or more wrappers and maps Lookup onto Record. The plugin never type-asserts a wrapper.

## How to use

- Open through `OpenBIN` or `OpenMMDB` with the plugin `New` context. Same config hash shares one file and one Updater.
- Those opens go through `Table[*BIN]` / `Table[*MMDB]` in this package (`std_go_reclaim.md`). Unreclaimed hash disposes after grace.
- IP2Location holds two BIN wrappers (geo + ASN). IPinfo and MaxMind each hold one MMDB wrapper.
- `provider.Close` must not close the shared wrapper.
- Tests call `dbwrappers.Reset`.

## Gotchas

- **Do** open the Lite MMDB with `os.ReadFile` + `maxminddb.FromBytes`. After `go mod vendor`, run `scripts/apply-oschwald-yaegi-patch.ps1` so Yaegi never loads upstream mmap / `x/sys` (`incomplete type ifreq`).

## Pattern snippet

```go
w, err := dbwrappers.OpenMMDB(ctx, dbwrappers.MMDBConfig{Source: src}, logger)
err = w.Lookup(ip, &rec)
```

## Key files

- `pkg/dbwrappers` — BIN, MMDB, `Table[T]`, `Reset`
- `pkg/dbsource` — Resolve and Updater used by both wrappers
