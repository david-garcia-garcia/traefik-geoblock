## Why

Operators cannot tell which catalog source a wrapper log belongs to. Resolve walks the whole plugin tree for a basename and logs the walk root, so `seeds/` never appears. ASN is not shipped under `seeds/` but still searches for `IP2LOCATION-LITE-ASN.IPV6.BIN` and ERRORS. A missing catalog `path` is silent.

## What Changes

- Wrapper and source logs include `key=<databaseSources map key>`. The updater does not also log `source`.
- Catalog `path` set and not a file: WARN `seed was specified but not found` plus that path, then continue Resolve.
- IP2Location ASN has no bundled `DefaultFileName`. No search for `IP2LOCATION-LITE-ASN.IPV6.BIN`. Empty path + no dated file → `no database file yet` only.
- `fileutils.Search` does not walk. `TRAEFIK_PLUGIN_GEOBLOCK_PATH` is the plugin root. Exact files only: `{env}/seeds/<name>`, then `{env}/<name>`. Configured path if it is an existing file.
- Env unset: say it must be set to the plugin root. Env set and both exact files missing: log the exact paths tried and that the env is probably not the plugin root.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_database_url-download`: Resolve uses exact plugin-root joins, not a basename walk. Missing catalog `path` WARNs. ASN has no bundled default name. Wrapper/source lines carry `key`.

## Impact

- `pkg/fileutils`, `pkg/dbsource`, `pkg/dbwrappers`, `pkg/ip2location/provider.go`, `plugin.go` ban HTML Search
- README `TRAEFIK_PLUGIN_GEOBLOCK_PATH`; `knowledge/devdocs/core_geoblock_database_source.md`
- Tests that relied on walking a subdirectory or placing a default file at the env dir root
