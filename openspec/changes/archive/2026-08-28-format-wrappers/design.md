## Context

See proposal.md — Why. Today IP2Location owns BIN lifecycle in `factory.go` (`DatabaseFactory` + `DatabaseWrapper`). IPinfo and MaxMind each copy MMDB `ReadFile` + `FromBytes` + mutex swap + `InstanceLock` into `provider`. File location and keep-current live in `pkg/dbsource` (renamed from `pkg/dbdownload`). Yaegi cannot type-assert a value stored as `any` in `dbprovider` (`InstanceLock` comment). Isolation spec currently puts open/hot-swap in the vendor package. The public catalog was never published, so `databaseDownloads` / `*_download_*` rename to `databaseSources` / `*_source_*`.

## Goals / Non-Goals

**Goals:**

- One package `pkg/dbwrappers` with two concrete format types (BIN, MMDB).
- Vendor `New` only gets wrappers and maps Lookup.
- Fold IP2Location Factory into the BIN type. Delete `factory.go`.
- Singleton maps stay concrete in `pkg/dbwrappers`.

**Non-Goals:**

- A generic `Wrapper[T]` or opener-callback orchestrator.
- Changing GET / unpack / dated-name / Resolve rules (package rename only).
- Renaming `databaseAutoUpdateDir` or `tools/dbdownload`.
- Deleting leftover `pkg/*/autoupdate.go` (noted on `devstate/issues.md`).
- Adding `pkg/bin` or `pkg/mmdb` as separate packages.

## Decisions

- **Package name `dbwrappers`.** Matches `dbprovider` / `dbsource`. Both formats live here so the third layer has one owner. Alternative: `pkg/mmdb` plus BIN left in ip2location — rejected; human asked for one package. Alternative: `pkg/dbwrapper` singular — rejected; two types.

- **Package rename `pkg/dbdownload` → `pkg/dbsource`.** The job is which file plus keep it current, not HTTP GET. Public Traefik keys: `databaseSources` (YAML camelCase) and `ip2location_source_geo` / `_asn`, `ipinfo_source`, `maxmind_source`. `tools/dbdownload` stays (seed CLI / `go:generate`). Alternative: `dbmanager` — rejected. Alternative: keep `databaseDownloads` — rejected; unpublished and “download” is the wrong word for seed-only rows.

- **Two types, not one interface used by the plugin.** BIN (`OpenDB`, temp copy, delayed close, `AllowMissing`, version) and MMDB (`FromBytes`, mutex, immediate close) do not share an open function. Providers call `Lookup` on BIN (geo/ASN SDK methods) or `Lookup(ip, dest any)` on MMDB. Alternative: shared orchestrator + hooks — rejected in explore (strategy bag).

- **Singleton per format config hash.** `map[string]*BIN` and `map[string]*MMDB` plus `InstanceLock` in `pkg/dbwrappers`. Alternative: map in each vendor package — duplicates MMDB. Alternative: `map[string]any` in `dbprovider` — Yaegi panic.

- **Provider is a new struct each `New`; wrapper is shared.** Matches today’s IP2Location (new provider, shared factory) and fixes IPinfo/MaxMind returning the same provider pointer. `Close` on the provider is a no-op for the wrapper. Tests reset via `dbwrappers` cleanup.

- **IP2Location `factory.go` deleted.** BIN type absorbs initialize, temp copy, hot-swap, ticker. Provider keeps `New` + merged Lookup.

- **No slot layer.** GEO and ASN are two independent wrapper + source pairs. The provider holds both. `dbsource.Updater` is the keep-current loop for one source, not a GEO/ASN role. Dated files use the catalog key (`DatedKeyGlob`). Alternative: reserved `geo`/`asn` slots — already rejected.

## Risks / Trade-offs

- [Yaegi loads `pkg/dbwrappers`] → same as other `pkg/` imports; keep maps concrete; MMDB still uses `FromBytes` (existing Yaegi mmap gotcha).
- [BIN temp files and delayed close move packages] → port behavior from `factory.go`; keep existing factory tests as wrapper tests.
- [Isolation spec said vendor-owned open] → delta updates that SHALL; plugin still never sees the wrapper type.

## Migration Plan

In-process refactor on `2.x`. Catalog keys change because they were never published. Rollback is revert the change.

## Open Questions

None.
