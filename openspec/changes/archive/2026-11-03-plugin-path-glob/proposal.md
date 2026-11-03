## Why

Traefik unpacks catalog plugins under `/plugins-storage/sources/gop-<random>/src/<module>`, so an exact `TRAEFIK_PLUGIN_GEOBLOCK_PATH` and the old unpack path fail at New. A row with a URL still fails startup when `defaultFile` is missing, before auto-update can GET the file.

## What Changes

- `fileutils.Search` accepts a glob in `TRAEFIK_PLUGIN_GEOBLOCK_PATH`. An exact directory still wins and skips glob. A set env (glob or exact) does not fall through to defaults. Unset env tries `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock` then `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock`.
- Several Glob matches: first sorted match that contains `seeds/<that call's defaultFile>`, else `<defaultFile>` at that directory. Traefik Manager owns `GoPath()`; Search only reconstructs from the operator env or those default globs.
- URL + missing file (including `defaultFile` set but not located) WARNs and starts for BIN and MMDB. `MMDBConfig` gains `AllowMissing`. Missing `defaultFile` with no URL still fails New.
- `banIfError` default stays `true`. Document `false` as fail-open on lookup only (`decide()` still applies CIDR, private, country lists, and `defaultAllow`).
- BundledFile miss when a dated file already won: DEBUG. Other Search misses that are the only resolution path: WARN.
- README documents `gop-*` under `/plugins-storage/sources` and keeps the `*` glob.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_database_url-download`: plugin-directory Search (glob, unset defaults, several matches); URL + missing file WARNs and starts for BIN and MMDB; miss log levels; `banIfError: false` is fail-open on lookup only.
- `core_geoblock_database_source-catalog`: restates `defaultFile` location so it does not disagree with url-download (no second `GoPath()` calculation).

## Impact

- `pkg/fileutils/fileutils.go` `Search` / `PluginPathEnv` (also ban HTML via `pkg/geoblock/config.go` `Prepare`)
- `pkg/dbsource/resolve.go` `BundledFile`
- `pkg/geoblock/plugin.go` `openCatalogRow` `AllowMissing`
- `pkg/dbwrappers/lifecycle.go` `initialize` / `chooseStartupTarget`
- `pkg/dbwrappers/bin.go` / `pkg/dbwrappers/mmdb.go` (`MMDBConfig.AllowMissing`)
- Native package tests for glob Search (`pkg/fileutils`). No Traefik-container glob test.
- README Local plugin / `TRAEFIK_PLUGIN_GEOBLOCK_PATH`; usage packet `knowledge/devdocs/core_geoblock_database_source.md` gotcha
- **Operators:** unset env and a `gop-*` glob can find the plugin tree. A URL-backed row can start with no on-disk file (WARN; lookups error until download). `banIfError: false` is not a policy bypass.
