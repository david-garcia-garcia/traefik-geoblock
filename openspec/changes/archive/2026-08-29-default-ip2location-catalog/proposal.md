## Why

An empty IP2Location geo pointer opens a bundled seed and never GETs. An empty MaxMind pointer opens the official dummy Country fixture and never GETs. Operators must copy download URLs into `databaseSources` by hand. A pointer to a missing catalog key, or a bound URL without `databaseAutoUpdateDir`, fails `New` and takes Traefik down. A BIN provider can be pointed at an `mmdb` row.

## What Changes

- Inject a reserved catalog key `default_ip2location` (free IP2Location geo LITE ZIP, `bin`/`zip`) in `New` when the operator did not define that key. Keep an operator-defined row. Document the reserved name.
- Empty `ip2location_source_geo` binds `default_ip2location` so the default URL can GET.
- Inject a reserved catalog key `default_geolite` (unofficial P3TERX Country MMDB, `mmdb`/`none`) in `New` when the operator did not define that key. Keep an operator-defined row. Do not commit a live GeoLite file.
- Empty `maxmind_source` binds `default_geolite` so that URL can GET. Until a dated file exists, Resolve still opens the official dummy Country seed.
- A geo (or other bound) pointer to a missing catalog key: WARN and treat as empty (IP2Location geo → `default_ip2location`; MaxMind → `default_geolite`; IPinfo → bundled seed; ASN → no ASN). Invalid `databaseProvider` still fails `New`.
- Bound URL with empty `databaseAutoUpdateDir`: WARN and use `filepath.Join(os.TempDir(), "traefik-geoblock")`.
- **BREAKING** for type: a bound pointer whose catalog `databaseType` does not match the provider format (IP2Location/`bin`, IPinfo|MaxMind/`mmdb`) fails `New`. Unknown `databaseType`/`archive` still fail `New`.
- No programmatic ASN default (token required). No City default.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_database_url-download`: reserved default catalog rows; missing pointer WARNs and falls back; empty IP2Location geo and MaxMind pointers bind the default rows (GET allowed); empty auto-update dir WARNs and uses a temp dir; pointer `databaseType` must match the provider.
- `core_geoblock_database_provider`: empty IP2Location geo pointer uses `default_ip2location`; empty MaxMind pointer uses `default_geolite` (bundled seed until a dated file exists).
- `core_geoblock_database_maxmind-geolite`: empty `maxmind_source` binds `default_geolite`; committed file stays the official dummy, not a live GeoLite download.

## Impact

- `plugin.go` — catalog inject, pointer bind/fallback, type check, temp dir
- `pkg/maxmind/edition.go` — `DefaultGeoliteURL`
- `plugin_config_test.go` — `MissingPointerKeyFails` and URL-without-dir cases
- `README.md` — document `default_ip2location` and `default_geolite`
- `knowledge/devdocs/core_geoblock_database_source.md` — usage contract
- Specs `core_geoblock_database_url-download`, `core_geoblock_database_provider`, `core_geoblock_database_maxmind-geolite`
