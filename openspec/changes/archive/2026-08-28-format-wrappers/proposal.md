## Why

IPinfo and MaxMind each copy the same MMDB open, hot-swap, and singleton. IP2Location keeps that lifecycle in a named Factory next to a BIN wrapper. The stack is not the same three layers, and the share axis is format (`bin` / `mmdb`), not vendor.

## What Changes

- Add `pkg/dbwrappers` with two format types: BIN and MMDB. Each owns resolve via `pkg/dbsource`, open, hot-swap, and a concrete singleton map.
- Rename `pkg/dbdownload` to `pkg/dbsource`. Rename unpublished catalog `databaseDownloads` to `databaseSources` and pointers `*_download_*` to `*_source_*`.
- Vendor providers become Lookup mapping only. IP2Location holds two BIN wrappers (geo + ASN). IPinfo and MaxMind each hold one MMDB wrapper.
- Remove `pkg/ip2location/factory.go`.
- Isolation: open and hot-swap live in `pkg/dbwrappers`. The plugin still MUST NOT type-assert a wrapper.
- `provider.Close` does not tear down a shared wrapper.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_database_provider`: isolation moves open/hot-swap to `pkg/dbwrappers`; IP2Location pointers are `ip2location_source_geo` / `_asn`.
- `core_geoblock_database_url-download`: catalog is `databaseSources`; pointers are `*_source_*`.
- `core_geoblock_database_ipinfo-lite`: pointer is `ipinfo_source`.
- `core_geoblock_database_maxmind-geolite`: pointer is `maxmind_source`.

## Impact

- `pkg/dbwrappers` (new)
- `pkg/dbsource` (renamed from `pkg/dbdownload`)
- Public Config: `databaseSources`, `ip2location_source_geo` / `_asn`, `ipinfo_source`, `maxmind_source`
- `pkg/ip2location` (thin provider; factory gone)
- `pkg/ipinfo`, `pkg/maxmind` (use MMDB wrapper)
- Specs: provider isolation + url-download / IPinfo / MaxMind pointer names
- `knowledge/devdocs/core_geoblock_database_provider.md`
- Tests that constructed `NewDatabaseFactory` / vendor-local singletons
