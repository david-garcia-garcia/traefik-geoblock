## Context

See proposal.md. `fileutils.Search` reads `TRAEFIK_PLUGIN_GEOBLOCK_PATH`, requires `ExistsAndIsDir`, and joins `seeds/<defaultFile>` then `<defaultFile>`. Unset env and a glob string both fail. Traefik’s plugin Manager owns `GoPath()` (`os.MkdirTemp(sourcesRootPath, "gop-*")` then `src/<module>`); that value is not given to plugin code. `openCatalogRow` sets BIN `AllowMissing` only when `Path=="" && DefaultFileName==""`. MMDB hardcodes `allowMissing: false`. `initialize` returns before `startUpdate` when `!allowMissing`. `chooseStartupTarget` `seedFirst` still calls `BundledFile` while Latest exists; Search `Error`s that miss. Yaegi v0.16.1 `filepath.Glob` already matched a middle-`*` layout (explore Reproduce).

## Goals / Non-Goals

**Goals:**

- Glob and unset defaults live in `fileutils.Search` (already owns `PluginPathEnv`).
- Reconstruct the on-disk plugin directory from the operator env or the asked default globs only.
- URL + missing file WARNs and starts for BIN and MMDB; no-URL miss still fails New.
- Miss log levels: DEBUG when dated already won; WARN when Search is the only path.
- Native package test for glob Search. README `gop-*` plus `banIfError: false` as fail-open on lookup only.

**Non-Goals:**

- A second `GoPath()` calculation (Yaegi internals, walking for a basename, narrowing `*` to `gop-*` only).
- Changing the `BanIfError` default, or a second “unfiltered until download” flag.
- A Traefik-container glob test (Yaegi interp already reproduced).
- BIN-only allow-missing.

## Decisions

- **Search is the seam.** Exact directory: `ExistsAndIsDir` wins and skips glob. Env set (glob or exact) is exclusive — zero directory matches do not fall through to unset defaults. Unset env tries `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock` then `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock`. Glob results that are not directories are skipped. Alternative: walk for a basename — rejected (live spec: exact joins). Alternative: read Manager `GoPath()` — rejected (not exported; Yaegi sees `os.Environ()`).
- **Several matches.** Keep the first `filepath.Glob` match (Glob sorts) that contains `seeds/<that call’s defaultFile>`; else the first that contains `<defaultFile>` at that directory (ban HTML is `geoblockban.html` at the directory, not under `seeds/`). None → same not-found as today. Search stays per-call. Two catalog rows picking two directories is accepted; Traefik normally has one `gop-*`.
- **Identity owner.** Traefik Manager owns `GoPath()`. Search reconstructs from the operator env or those default globs. `dbsource` / catalog specs MUST call Search, not re-derive the directory.
- **Wait-for-URL.** `openCatalogRow` sets allow-missing when the row has a non-empty URL **or** (`Path=="" && DefaultFileName==""`) so ASN empty-both stays optional. `MMDBConfig` gains `AllowMissing`; `newMMDB` uses it. `initialize` WARNs (`no database file yet; waiting for auto-update`, today Info) and returns; `startUpdate` still runs. Missing `defaultFile` with no URL stays `!allowMissing` and fails. Nil-handle errors stay `BIN is not open` / `MMDB is not open`.
- **`banIfError`.** Do not change `CreateConfig` default (`true`). Document the existing branch: while the handle is nil, lookups error; `true` bans (`block:error`); `false` skips the `lookupFailed` / empty-country bans and continues into `decide()` — CIDR, private, country lists, and `defaultAllow` still apply. Fail-open on lookup only. README + url-download.
- **Miss logs.** Search MUST NOT log Error on a miss. File miss: DEBUG from Search. Exclusive glob/directory miss: WARN. `initialize` WARNs when starting with no file (wait) or returns the error that fails New (only path). `chooseStartupTarget` DEBUGs a BundledFile miss when Latest already won. Prepare ban-HTML miss WARNs if the path is empty.
- **Tests.** Native `pkg/fileutils` cases: env glob with a temp `gop-*` layout; exact directory skips glob; env glob with zero matches does not fall through; several matches prefer `seeds/<defaultFile>`. No Traefik-container glob. Wrapper tests: BIN and MMDB URL + missing `defaultFile` start; no-URL miss still fails.
- **Docs.** README names `gop-*` under `/plugins-storage/sources` and the full `{gop-*}/src/github.com/david-garcia-garcia/traefik-geoblock` shape. Keep the asked default glob `*`. Exact `/plugins-local/...` stays valid. Usage packet `core_geoblock_database_source.md` gotcha follows url-download (unset tries defaults; glob accepted).

## Risks / Trade-offs

- [Several `gop-*` leftovers on disk] → First lexicographic match that contains this call’s `defaultFile` wins. Traefik `resetDirectory` on sources at `NewManager` usually leaves one. Two rows with two names may pick two directories — accepted.
- [Env glob exclusive miss while `/plugins-local` would have worked] → Operator set the env; do not override it. Exact directory still works.
- [`banIfError: false` while the handle is nil] → Requests are not banned for lookup failure; CIDR / country policy still runs. Default stays true. Document, do not add a bypass flag.
- [Hardcoded unset paths are Unix] → Required glob test uses `t.TempDir()` + env glob, not `/plugins-storage` on the developer host.

## Migration Plan

- Deploy the new plugin binary. Exact `TRAEFIK_PLUGIN_GEOBLOCK_PATH` directories keep working.
- Operators who omit the env get the default globs. Operators who set a glob get only that glob.
- URL-backed rows can start with no on-disk file (WARN). Operators who need fail-closed until a file exists keep `banIfError: true` (default).

## Open Questions

None — rows live on `devstate/explore.md`.
