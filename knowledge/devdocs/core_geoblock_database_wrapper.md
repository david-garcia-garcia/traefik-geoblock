# Wrapper

## Language

**Wrapper**:
One open geo database file of one format (BIN or MMDB): resolve, open, hot-swap, singleton-per-config.
_Avoid_: Factory, vendor brand, slot

**Format**:
How the bytes are opened (`bin` / `mmdb`). IPinfo and MaxMind share MMDB; they differ only in field tags.
_Avoid_: treating vendor as the share axis

## Overview

`pkg/dbwrappers` owns BIN and MMDB open/hot-swap and the vendor field maps (`BINSource`, `IPinfo`, `GeoIP2`). The plugin never type-asserts a wrapper. Maps stay in code (BIN `Get_all` / `Get_asn`, IPinfo tags, GeoIP2 tags). Catalog `fields` is `Record.Keep` after Lookup.

## How to use

- Open through `OpenBIN` or `OpenMMDB` with the plugin `New` context. Same config hash shares one file and one Updater. The wrapper logger is scoped with `key` equal to the catalog map key.
- Those opens go through `reclaim.Open` (`std_go_reclaim.md`) with `bin:` / `mmdb:` keys. Create watches the incarnation lifetime and calls `close` when it is canceled. Unreclaimed hash ends after grace. The caller asserts `*BIN` / `*MMDB`, then wraps with `NewBINSource` / `NewIPinfo` / `NewGeoIP2` and the row’s `fields`.
- One catalog row is one wrapper. IP2Location geo and ASN LITE are two `ip2location` rows; the ASN row sets `fields: [asn]` so Lookup calls `Get_asn` only.
- Source `Close` must not close the shared wrapper.
- Tests call `dbwrappers.Reset`. Short-grace plugin tests call `ResetWith`.

## Gotchas

- **Do** open the Lite MMDB with `os.ReadFile` + `maxminddb.FromBytes`. After `go mod vendor`, run `scripts/apply-oschwald-yaegi-patch.ps1` so Yaegi never loads upstream mmap / `x/sys` (`incomplete type ifreq`).

## Pattern snippet

```go
w, err := dbwrappers.OpenMMDB(ctx, dbwrappers.MMDBConfig{Source: src}, logger)
src := dbwrappers.NewIPinfo(w, nil)
rec, err := src.Lookup(ip)
```

## Key files

- `pkg/dbwrappers` — BIN, MMDB, field maps, `Reset`
- `pkg/reclaim` — process table (`any`)
- `pkg/dbsource` — Resolve and Updater used by both wrappers
