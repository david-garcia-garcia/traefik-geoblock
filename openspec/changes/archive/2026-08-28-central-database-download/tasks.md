## 1. Download component

- [x] 1.1 Add `pkg/dbdownload` (or equivalent): GET via `dbutils.HTTPGet`, lock, temp, unpack by `archive` (`none`/`zip`/`tar.gz`), date by `databaseType` (`bin`/`mmdb`), dated `YYYYMMDD_<key>.<ext>`, “already current”, ticker, find-latest. Never log the URL. Cadence is an argument.
- [x] 1.2 Unit-test unpack + date + dated name with `httptest` (zip-bin, raw mmdb, tar.gz-mmdb). Hint omits URL. Empty archive + extension-less URL fails without sniffing.

## 2. Config surface

- [x] 2.1 Catalog values are `{ url, headers, databaseType, archive }`. Free keys. `CreateConfig` initializes the map. Pointers: `ip2location_download_geo`, `ip2location_download_asn`, `ipinfo_download`, `maxmind_download`.
- [x] 2.2 Remove reserved `geo`/`asn` slot validation. Fail `New` on unknown `databaseType`/`archive`, pointer to a missing key, or bound URL without `databaseAutoUpdateDir`. Empty pointer + seed succeeds.
- [x] 2.3 Keep `databaseFilePath` → `ip2location_databaseFilePath` only. No token/code keys.

## 3. Providers instantiate the component

- [x] 3.1 IP2Location: two component instances from the two pointers. Delete vendor download loop. Keep BIN open, local copy, singleton, hot-swap in `factory.go`.
- [x] 3.2 IPinfo: one component from `ipinfo_download`. Delete `autoupdate.go` loop. Keep MMDB Lookup open/hot-swap. Bundled `ipinfo_lite.mmdb` seed.
- [x] 3.3 MaxMind: one component from `maxmind_download`. Delete `autoupdate.go` loop. Keep dummy Country seed and GeoIP2 Lookup.

## 4. Docs and compose

- [x] 4.1 README: catalog + `databaseType`/`archive` + pointer examples (lite ZIP, token `file=` URL, IPinfo `?token=`, MaxMind Basic). No leftover token/code keys or reserved-only `geo`/`asn` docs.
- [x] 4.2 Rewrite compose / Pester labels to catalog + pointers + shared dir.
- [x] 4.3 Update `knowledge/devdocs/core_geoblock_database_provider.md`.

## 5. Tests

- [x] 5.1 Config tests: missing pointer key fails; unknown type/archive fails; URL without dir fails; empty pointers + seed succeed; file-path alias still works.
- [x] 5.2 Provider tests: httptest through the component; empty pointer keeps seed; no builder tests for token/code URLs.
- [x] 5.3 `go test ./...`
