# Requirement
IssueKey: 2026-11-03-plugin-path-glob

## Problem

1. Find the plugin folder without a fixed path. Accept a glob in TRAEFIK_PLUGIN_GEOBLOCK_PATH (example: /plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock). If the variable is unset, try by default that glob and /plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock. If several matches, keep the one that contains seeds/<defaultFile>. filepath.Glob is Go stdlib and should work under Traefik Yaegi, but it must be tested. Compatible with current configurations (an exact directory path must still work).
2. If there is no database file but the source has a URL, wait for the download instead of failing startup. Start without a database with a WARN and let auto-update download it. Meanwhile lookups return an error (pkg/dbwrappers/bin.go around the nil-handle path), and request fate depends on banIfError: true blocks, false passes unfiltered. That false path is a security decision and must be decided and documented. Update openspec/specs/core_geoblock_database_url-download, which today requires startup to fail.
   Additional, same change: if defaultFile is set but that file cannot be located, AND the source has a URL, log a WARN and continue. Do not refuse to start. The URL is how a file is obtained later. Today a non-empty missing defaultFile sets AllowMissing false and New fails before the updater starts (pkg/geoblock/plugin.go AllowMissing; pkg/dbwrappers/lifecycle.go initialize; pkg/fileutils/fileutils.go Search). A missing defaultFile with NO url stays a startup failure. This applies to every source type that can have a URL, not only BIN.
3. Lower the BundledFile error to DEBUG or WARN when a downloaded file already exists.
4. Update the README with Traefik's new plugin folder (gop-* under /plugins-storage/sources).

## Current (code)

- Plugin root is an exact directory in `TRAEFIK_PLUGIN_GEOBLOCK_PATH`. `pkg/fileutils/fileutils.go` `Search` reads `PluginPathEnv`, requires `ExistsAndIsDir`, and joins `seeds/<defaultFile>` then `<defaultFile>`. Unset env is an ERROR. A glob string is not a directory, so it fails the same check. No `filepath.Glob` on this path (`filepath.Glob` exists only in `pkg/dbutils/dbutils.go` for dated catalog files).
- Unset env has no fallback to `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock` or `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock`. Same `Search`.
- Several plugin-root candidates: `not found` (no disambiguation by `seeds/<defaultFile>`).
- Missing `defaultFile` with a URL still fails New: `pkg/geoblock/plugin.go` `openCatalogRow` sets BIN `AllowMissing` only when `src.Path == "" && src.DefaultFileName == ""` (URL is not in that predicate). `pkg/dbwrappers/lifecycle.go` `initialize` returns `failed to resolve database path` / `database file not found` when `!allowMissing`. `newBIN` / `newMMDB` call `initialize` before `startUpdate`.
- MMDB never allows a missing file: `pkg/dbwrappers/mmdb.go` `newMMDB` hardcodes `allowMissing: false`. `MMDBConfig` has no `AllowMissing` field. A URL-only MMDB row (reserved `default_geolite` in `pkg/geoblock/config.go` `insertReservedCatalog`) cannot start without an on-disk file.
- `Search` of a missing `defaultFile` is ERROR then empty: `pkg/fileutils/fileutils.go` `Search`; `pkg/dbsource/resolve.go` `BundledFile` returns that error. A missing `defaultFile` with no URL stays a startup failure through the same `initialize` path.
- Nil-handle lookups already error: `pkg/dbwrappers/bin.go` `getAll` (`db == nil` → `BIN is not open`); `pkg/dbwrappers/mmdb.go` `Lookup` (`db == nil` → `MMDB is not open`). Request fate: `pkg/geoblock/plugin.go` `ServeHTTP` `lookupFailed && p.banIfError` blocks; `CreateConfig` defaults `BanIfError: true` (`pkg/geoblock/config.go`).
- Live spec: `openspec/specs/core_geoblock_database_url-download/spec.md` — env unset / exact files missing must be logged as not the plugin root; until a dated file exists, Resolve still opens `{env}/seeds/<defaultFile>` when that file exists. ASN with empty `defaultFile` is the documented wait-for-update case. There is no scenario that starts with a URL, a missing `defaultFile`, and a WARN.
- BundledFile ERROR while a dated file already exists: `pkg/dbwrappers/lifecycle.go` `chooseStartupTarget` `seedFirst` calls `dbsource.BundledFile`; that calls `Search`, which `logger.Error`s `bundled file not found` even when `Latest` is the resolved path.
- README plugin folder: `README.md` Local plugin — exact `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock` and catalog unpack `/plugins-storage/sources/github.com/david-garcia-garcia/traefik-geoblock`. No `gop-*` under `/plugins-storage/sources`.

## Desired

- `TRAEFIK_PLUGIN_GEOBLOCK_PATH` accepts a glob. Unset: try `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock` then `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock`. Several matches: keep the one that contains `seeds/<defaultFile>`. An exact directory path still works. `filepath.Glob` must be proven under Traefik Yaegi.
- No database file, source has a URL: do not fail startup. WARN and let auto-update download. Lookups error on the nil-handle path; `banIfError: true` blocks, `false` passes unfiltered. Decide and document that false path. Update `openspec/specs/core_geoblock_database_url-download` (today’s contract does not allow this start).
- `defaultFile` set, file not located, source has a URL: WARN and continue. Same for every source type that can have a URL, not only BIN. Missing `defaultFile` with no URL stays a startup failure.
- When a downloaded file already exists, BundledFile missing-seed is DEBUG or WARN, not ERROR.
- README documents Traefik’s `gop-*` folder under `/plugins-storage/sources`.

## Affected

- `pkg/fileutils/fileutils.go` `Search` / `PluginPathEnv` (also ban HTML via `pkg/geoblock/config.go` `Prepare`)
- `pkg/dbsource/resolve.go` `BundledFile`
- `pkg/geoblock/plugin.go` `openCatalogRow` `AllowMissing`
- `pkg/dbwrappers/lifecycle.go` `initialize` / `chooseStartupTarget`
- `pkg/dbwrappers/bin.go` `AllowMissing` / nil-handle lookup
- `pkg/dbwrappers/mmdb.go` hardcoded `allowMissing: false`
- `openspec/specs/core_geoblock_database_url-download/spec.md`
- `README.md` Local plugin / `TRAEFIK_PLUGIN_GEOBLOCK_PATH`

## Out of scope

- Changing the `BanIfError` default (`true` in `CreateConfig`).
- Walking the plugin tree for a basename (live spec: exact joins only).
- Building vendor download URLs.
- Changing CIDR / country policy, or the nil-handle lookup error itself (reuse it).
- Inventing a Traefik plugin-path layout other than the glob, the local fallback, and `gop-*` docs named in the ask.

## Unknowns

- Does `filepath.Glob` work under Traefik Yaegi for `/plugins-storage/sources/*/…`?
- When several glob matches contain `seeds/<defaultFile>`, or sources disagree on `defaultFile`, which root wins?
- BundledFile missing-seed with a dated file present: DEBUG or WARN?
- `banIfError: false` while the database is missing: fail-open is a security decision; which documented operator default / warning is required?
- Exact on-disk `gop-*` shape on current Traefik versus the example glob (`*` vs `gop-*`).

## Tensions

- Ask: unset env tries default globs. Code/spec: unset env ERROR “must be set to the plugin root” (`Search`; url-download Env unset scenario).
- Ask: missing `defaultFile` + URL → WARN and start. Code: non-empty `DefaultFileName` forces `AllowMissing` false; MMDB hardcodes `allowMissing` false; `initialize` fails before `startUpdate`. Spec: missing exact files log “probably not the plugin root”; wait-without-file is specified only for ASN with no bundled `defaultFile`.
- Ask: `banIfError: false` passes unfiltered until download; that path must be decided. Code already implements that branch for lookup errors; default is `true`. No operator doc for “no DB yet”.
- Ask: BundledFile ERROR → DEBUG or WARN when a downloaded file exists. Code: `Search` always ERROR on miss, including `seedFirst` while `Latest` exists.
- Ask: README `gop-*` under `/plugins-storage/sources`. README still shows the old exact unpack path without `gop-*`.
