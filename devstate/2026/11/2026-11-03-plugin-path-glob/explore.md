# Explore

## Concepts

Plugin-root lookup and catalog open are two jobs. Traefik already chose the on-disk plugin tree; this plugin reconstructs that directory so `seeds/<defaultFile>` and `geoblockban.html` can be joined. Catalog open then decides whether a missing file is fatal or a wait-for-GET.

```
Traefik Manager.GoPath()          operator env
{output}/sources/gop-*/src/<mod>  TRAEFIK_PLUGIN_GEOBLOCK_PATH
        \                        /
         \                      /
          v                    v
     fileutils.Search  (plugin root: exact dir, glob, or unset defaults)
          |                    |
          |                    +-- Prepare ban HTML (geoblockban.html)
          v
     dbsource.BundledFile / Resolve
          v
     lifecycle.initialize  --fail--> New  (today, before startUpdate)
          |
          +-- allowMissing: Info "no database file yet" then startUpdate
          +-- seedFirst: BundledFile even when Latest already won
          v
     BIN/MMDB nil handle  --> Lookup error
          v
     ServeHTTP: lookupFailed && banIfError → block; false continues into decide()
```

### Units this change would touch

| Unit | Path | Job |
|------|------|-----|
| Search | `pkg/fileutils/fileutils.go` | Read `TRAEFIK_PLUGIN_GEOBLOCK_PATH`, require a directory, join `seeds/<defaultFile>` then `<defaultFile>`. Owns plugin-root. |
| BundledFile / Resolve | `pkg/dbsource/resolve.go` | Dated Latest, then catalog `path`, then Search. |
| openCatalogRow | `pkg/geoblock/plugin.go` | BIN `AllowMissing: Path=="" && DefaultFileName==""` (URL is not in the predicate). MMDB has no such flag. |
| initialize / chooseStartupTarget | `pkg/dbwrappers/lifecycle.go` | Fatal `failed to resolve database path` / `database file not found` when `!allowMissing`, before `startUpdate`. seedFirst calls BundledFile while Latest exists. |
| BIN | `pkg/dbwrappers/bin.go` | `AllowMissing` on config; nil handle → `BIN is not open`. |
| MMDB | `pkg/dbwrappers/mmdb.go` | `allowMissing: false` hardcoded; `MMDBConfig` has no field. Nil handle → `MMDB is not open`. |
| CreateConfig / ServeHTTP | `pkg/geoblock/config.go`, `plugin.go` | `BanIfError` default true; lookupFailed / empty country use that flag, then `decide()` still applies CIDR / private / country / `defaultAllow`. |
| Live spec | `openspec/specs/core_geoblock_database_url-download/spec.md` | Env unset must log “set to the plugin root”; missing exact files log “probably not the plugin root”; wait-without-file is specified only for ASN with empty `defaultFile`. |
| README | `README.md` Local plugin | Exact `/plugins-local/...` and catalog unpack `/plugins-storage/sources/github.com/...` (no `gop-*`). |
| Usage | `knowledge/devdocs/core_geoblock_database_source.md` | Same env-must-be-set gotcha. |

### Call sites

Roots searched: `pkg/**/*.go`, `plugin.go`, `README.md`, `openspec/specs/**`.

- `fileutils.Search` / `Default.Search`: **2** production — `pkg/dbsource/resolve.go` `BundledFile`; `pkg/geoblock/config.go` `Prepare` (ban HTML). Package wrapper at `pkg/fileutils/fileutils.go`.
- `OpenBIN` production: **1** — `openCatalogRow`.
- `OpenMMDB` production: **1** — `openCatalogRow`.
- `AllowMissing` assignment: **1** (`plugin.go`); MMDB hardcode: **1** (`mmdb.go` `newMMDB`).
- `filepath.Glob` production today: **1** — `pkg/dbutils/dbutils.go` `FindLatestDatedFile` (dated catalog names, not plugin root).

### Reproduce

Run on this worktree, 2026-11-03. Throwaway `pkg/dbwrappers` tests deleted after the run.

| Claim | Result | What ran |
|-------|--------|----------|
| Missing `defaultFile` + URL fails New/open | **reproduced** | `OpenBIN` `AllowMissing: false` (the `openCatalogRow` predicate when `DefaultFileName` is set), empty plugin dir, URL `https://example.invalid/IP2LOCATION-LITE-DB1.IPV6.BIN`. Error: `failed to resolve database path: file not found: IP2LOCATION-LITE-DB1.IPV6.BIN`. Search ERROR: `bundled file not found; TRAEFIK_PLUGIN_GEOBLOCK_PATH is probably not the plugin root`. |
| Same at plugin `New` (reserved BIN row has URL + `defaultFile`) | **reproduced** | `go test ./pkg/geoblock -run TestNew/NoFilePath_NoEnvironmentVariable_ShouldError`. ERROR `TRAEFIK_PLUGIN_GEOBLOCK_PATH must be set to the plugin root` `key=default_ip2location`. |
| MMDB URL, no on-disk file | **reproduced** | `OpenMMDB` URL `https://example.invalid/GeoLite2-Country.mmdb`, empty plugin dir. Error: `database file not found`. (`newMMDB` `allowMissing: false`; `startUpdate` not reached.) |
| Missing `defaultFile`, no URL, stays fatal | **reproduced** | `OpenBIN` `AllowMissing: false`, no URL, empty plugin dir. Same `failed to resolve database path: file not found`. |
| Glob string is not a directory | **reproduced** | `Search` with env `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock`. Error: `TRAEFIK_PLUGIN_GEOBLOCK_PATH is not a directory`. |
| Unset env is ERROR | **reproduced** | `go test ./pkg/fileutils -run TestFileUtils_Search/empty_base_path_returns_error_without_env_var` — `TRAEFIK_PLUGIN_GEOBLOCK_PATH must be set to the plugin root`. |
| BundledFile miss ERROR while dated file exists | **reproduced** | `OpenBIN` with dated `YYYYMMDD_probe_seed.BIN` in `Dir` and no seed under env. Log contains `bundled file not found`. Open itself succeeded (`seedFirst` still calls `BundledFile`). |
| Wrapper can already wait | **reproduced** (existing green) | `go test ./pkg/dbwrappers -run TestOpenBIN_AllowMissing` — `AllowMissing: true` starts with no file; lookup errors `BIN is not open`. |
| `filepath.Glob` middle-`*` (native) | **reproduced** | `filepath.Glob({tmp}/*/src/github.com/david-garcia-garcia/traefik-geoblock)` returned the `gop-12345` plugin dir. |
| `filepath.Glob` under Yaegi | **reproduced** (interp, not a Traefik container) | Throwaway module `%TEMP%\yaegi-glob-probe`: Yaegi **v0.16.1** `interp` + `stdlib.Symbols` (the Traefik plugin interpreter stack per `knowledge/research/ext_traefik_plugins_yaegi-subpackages`). `filepath.Glob` on `{tmp}/*/src/github.com/david-garcia-garcia/traefik-geoblock` returned the `gop-12345` dir, `err=<nil>`. Live Traefik process: not run. |

### Outside facts used

- Traefik plugin Manager (`traefik/traefik` `pkg/plugins/manager.go`, GitHub contents at sha `38d53dd06c53c7e9bd08e57b217e72686d823610`): `os.MkdirTemp(sourcesRootPath, "gop-*")`; sources = `{goPath}/src`; `GoPath()` is that temp dir. Layout: `{plugins-storage}/sources/gop-<random>/src/<module>`. `resetDirectory` on sources at `NewManager`, so one `gop-*` per process in the usual case. Local GOPATH remains `./plugins-local/` (`knowledge/research/ext_traefik_plugins_yaegi-subpackages`).
- Traefik Yaegi loads stdlib symbols into the interpreter (`middlewareyaegi.go` extract in that same packet).
- In-tree: `core_geoblock_database_source.md`, `core_geoblock_database_wrapper.md`, `core_geoblock_plugin_packages.md`. No new `knowledge/` write this phase.

## Decisions

- **Seam:** glob and unset defaults live in `fileutils.Search` (already owns `PluginPathEnv`). Exact directory: `ExistsAndIsDir` wins and skips glob. Env set (glob or exact) is exclusive — zero matches do not fall through to the unset defaults. Unset env tries `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock` then `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock`.
- **Several glob matches:** keep the first `filepath.Glob` match (Glob sorts) that contains `seeds/<defaultFile>`; else the first that contains `<defaultFile>` at the plugin root (ban HTML is `geoblockban.html` at root, not under `seeds/`). None → same not-found as today.
- **Wait-for-URL:** `openCatalogRow` sets allow-missing when the row has a non-empty URL (BIN and MMDB). `MMDBConfig` gains `AllowMissing` like BIN (symmetry). `initialize` WARNs (`no database file yet; waiting for auto-update` today is Info) and returns; `startUpdate` still runs. Missing `defaultFile` with no URL stays `!allowMissing` and fails. Nil-handle lookup errors stay as they are (`BIN is not open` / `MMDB is not open`).
- **`banIfError`:** do not change the `CreateConfig` default (`true`, out of scope). Document the existing branch: while the handle is nil, lookups error; `true` bans (`block:error`); `false` skips the lookupFailed / empty-country bans and continues into `decide()` — CIDR, private, country lists, and `defaultAllow` still apply. That is fail-open on **lookup**, not a policy bypass. README + `core_geoblock_database_url-download`.
- **BundledFile miss with Latest already chosen:** DEBUG, not ERROR. Fatal vs optional is the caller’s job (`initialize` return); Search used as an optional seed-first peek must not Error. Other Search misses that remain the only resolution path: WARN (the returned error still fails New when `!allowMissing`).
- **Spec:** change `openspec/specs/core_geoblock_database_url-download` so URL + missing file / missing `defaultFile` WARNs and starts; env unset tries default globs before “must be set”; keep no-URL miss as a startup failure; add BIN and MMDB. `core_geoblock_database_source-catalog` restates the env joins — delta there so the two specs do not disagree. Usage packet `core_geoblock_database_source.md` gotcha follows.
- **README:** document Traefik’s `gop-*` under `/plugins-storage/sources` (`{dir}/sources/gop-<random>/src/github.com/david-garcia-garcia/traefik-geoblock`). Keep the asked default glob with `*`. Exact `/plugins-local/...` stays valid.
- **Yaegi Glob:** treat as proven under Yaegi v0.16.1 stdlib symbols. Implement still adds a package test for glob Search (native). A Traefik-container glob is not required for propose.

Rejected:

- Walking the plugin tree for a basename (out of scope; live spec: exact joins).
- Reading Traefik `GoPath()` from inside the plugin (not exported; Yaegi only sees `os.Environ()`).
- Narrowing the default glob to `gop-*` (ask names `*`; `*` already matches `gop-*`).
- Changing `BanIfError` default, or adding a second “unfiltered until download” flag.
- BIN-only allow-missing (ask: every source type that can have a URL).
- Inventing a plugin-path layout other than the named glob, local fallback, and `gop-*` docs.

Live contract: `openspec/specs/core_geoblock_database_url-download` (must change). Related: `core_geoblock_database_source-catalog`. Usage: `knowledge/devdocs/core_geoblock_database_source.md`.

## Open questions

- Q: Who already owns the on-disk plugin-root path that `Search` reconstructs from `TRAEFIK_PLUGIN_GEOBLOCK_PATH`?
  Rank: bounded asked — existing Search contract (must be a directory); 2 production call sites enumerated (`BundledFile`, `Prepare` ban HTML), roots `pkg/**`; criterion 1 names find the plugin folder without a fixed path
  Decision: assumed — Traefik’s plugin Manager owns `GoPath()` (`os.MkdirTemp(sourcesRootPath, "gop-*")` then `src/<module>`). That value is not given to plugin code. Reconstruct it from the operator env (exact directory still works) or the asked default globs that match Manager layout and `./plugins-local/`. Do not re-derive it from a peer library or from Yaegi internals.
  By: explore

- Q: When several glob matches all contain `seeds/<defaultFile>`, or two catalog rows pass different `defaultFile` names, which plugin root wins?
  Rank: additive asked — new disambiguation this change adds; criterion 1 names several matches keep the one containing `seeds/<defaultFile>`
  Decision: assumed — first sorted Glob match that contains `seeds/<that call’s defaultFile>`, else `<defaultFile>` at root, else not-found. Search stays per-call (ban HTML uses `geoblockban.html`). Two rows picking two roots is accepted; Traefik normally has one `gop-*`.
  By: explore

- Q: BundledFile missing-seed while a dated file is already the resolved path: DEBUG or WARN?
  Rank: additive asked — log-level change on an existing miss path; criterion 3 names DEBUG or WARN
  Decision: assumed — DEBUG. The operator already has a working dated file; WARN implies action. `initialize`’s return remains the fatal signal when there is no URL.
  By: explore

- Q: What operator contract is required for `banIfError: false` while the database is missing?
  Rank: additive asked — documentation of an existing branch; criterion 2 names decide and document; Out of scope: changing the `BanIfError` default
  Decision: assumed — default stays true (fail-closed until a handle exists). `false` is fail-open on lookup errors only: `ServeHTTP` does not ban for `lookupFailed` / empty country, then `decide()` still applies CIDR, private, country lists, and `defaultAllow`. Document that in README and url-download. Do not add a new bypass flag.
  By: explore

- Q: Exact on-disk `gop-*` shape versus the example glob (`*` vs `gop-*`)?
  Rank: additive asked — README wording; criterion 4 names `gop-*` docs; Out of scope: inventing a layout other than the named glob, local fallback, and `gop-*` docs
  Decision: assumed — keep the asked default glob `*` (it matches Traefik `MkdirTemp(..., "gop-*")`). README names the folder `gop-*` under `/plugins-storage/sources` and the full `{gop-*}/src/github.com/david-garcia-garcia/traefik-geoblock` shape. Do not change the glob to `gop-*` only.
  By: explore

- Q: Does `filepath.Glob` work under Traefik Yaegi for `/plugins-storage/sources/*/…`?
  Rank: additive asked — criterion 1 names filepath.Glob must be tested
  Decision: resolved — Yaegi v0.16.1 `interp` + `stdlib.Symbols` returned the `gop-12345` plugin dir for a middle-`*` pattern; native `filepath.Glob` matched the same layout; `pkg/dbutils` already calls `filepath.Glob` in production. Live Traefik container not run.
  By: explore
