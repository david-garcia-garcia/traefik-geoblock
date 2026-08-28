## 1. Download Resolve

- [x] 1.1 Add `path` and `DefaultFileName` on `dbdownload.Config`. Add `Resolve` (Latest, then existing `path` file, then `fileutils.Search` / `TRAEFIK_PLUGIN_GEOBLOCK_PATH`).
- [x] 1.2 Unit-test Resolve: dated file wins; catalog path file; empty path finds env default; empty key skips Latest.

## 2. Config surface

- [x] 2.1 Add `path` on `DatabaseDownload`. Pass it through `catalogDownload`. Remove `Ip2locationDatabaseFilePath`, `Ip2locationAsnDatabaseFilePath`, `IpinfoDatabaseFilePath`, `MaxmindDatabaseFilePath`, `DatabaseFilePath`, and `applyDeprecatedIP2LocationSettings`.
- [x] 2.2 Pass vendor default filenames into each download slot. ASN with empty pointer stays `AllowMissing`.

## 3. Providers use Resolve

- [x] 3.1 IP2Location factory: open `dbdownload.Resolve`; delete `resolveDatabasePath` seed search.
- [x] 3.2 IPinfo: open Resolve; delete `resolveSeedPath`.
- [x] 3.3 MaxMind: open Resolve; delete `resolveSeedPath`.

## 4. Docs and compose

- [x] 4.1 README: seed keys gone; catalog `path` + pointer; keep env default order.
- [x] 4.2 Compose / Pester / `.env.example` / `.traefik.yml` call sites.
- [x] 4.3 Update `knowledge/devdocs/core_geoblock_database_provider.md`.

## 5. Tests

- [x] 5.1 Rewrite config and provider tests off `*_databaseFilePath` / `databaseFilePath`. Cover catalog `path` + pointer; empty pointer + env default; no alias.
- [x] 5.2 `go test ./...`
