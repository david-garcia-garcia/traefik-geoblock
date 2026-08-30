## Why

Operators pick one `databaseProvider` and bind catalog rows with vendor pointers. Field maps and bundled seed names live in that vendor package, so two vendors cannot fill one Record and default `seeds/` files disappear if the provider layer is removed.

## What Changes

- **BREAKING:** Remove Config `databaseProvider`, `ip2location_source_geo`, `ip2location_source_asn`, `ipinfo_source`, and `maxmind_source`. Lookup opens **enabled** `databaseSources` rows.
- Each catalog row names `vendor` (`ip2location` | `ip2location-asn` | `ipinfo` | `maxmind`) and optional `defaultFile` (basename under `seeds/`). `databaseType` stays format only (`bin` / `mmdb`).
- `enabled` on a row: omitted means enabled. Shipped extras that must not all run are inserted `enabled: false`.
- Plugin creation inserts reserved catalog rows when the operator did not define that key: `default_ip2location` (enabled; free LITE ZIP + `IP2LOCATION-LITE-DB1.IPV6.BIN`), `default_ipinfo` (disabled; `ipinfo_lite.mmdb`), `default_maxmind` (disabled; `GeoIP2-Country-Test.mmdb`), `default_geolite` (disabled; unofficial Country GET). Operator-defined reserved keys are kept.
- Enrich Lookup merges enabled sources in lexicographic catalog-key order: first non-empty value per meta key wins.
- Wrappers stay. One internal `Provider` still exposes Lookup to the plugin.

## Capabilities

### New Capabilities

- `core_geoblock_database_source-catalog`: Catalog is the operator model; vendor, defaultFile, enabled; merge N sources; shipped seed and download rows.

### Modified Capabilities

- `core_geoblock_database_provider`: Lookup is the merged enabled sources, not one `databaseProvider`.
- `core_geoblock_database_url-download`: Drop pointers; `defaultFile` on the row; reserved seed rows; auto-update for enabled URLs.
- `core_geoblock_database_ipinfo-lite`: Bind by catalog `vendor: ipinfo`, not `ipinfo_source`.
- `core_geoblock_database_maxmind-geolite`: Bind by catalog `vendor: maxmind`, not `maxmind_source`.
- `core_geoblock_plugin_request-mode`: Lookup modes open enabled sources, not a selected provider.
- `core_geoblock_plugin_instance-reclaim`: Incarnation opens enabled sources only in lookup modes.

## Impact

- `pkg/geoblock/config.go` — catalog fields; remove provider pointers; shipped rows in `Prepare`.
- `pkg/geoblock/plugin.go` — open and merge enabled sources.
- `pkg/dbwrappers` — vendor field maps (`BINSource`, `ASNSource`, `IPinfo`, `GeoIP2`).
- `pkg/geoblock` — shipped seed/URL constants on catalog insert. Vendor packages removed.
- `pkg/dbsource` — `DefaultFileName` from the catalog row.
- `plugin.go` — Yaegi `Config` alias.
- Tests, README, compose, `.traefik.yml`.
