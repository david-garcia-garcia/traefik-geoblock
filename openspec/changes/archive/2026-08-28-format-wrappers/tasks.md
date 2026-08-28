## 1. Format wrappers

- [x] 1.1 Add `pkg/dbwrappers` with MMDB type: `FromBytes` open, mutex swap, `dbsource.Resolve` / `Start`, singleton `map[string]*MMDB`, `Lookup(ip, dest any)`.
- [x] 1.2 Add BIN type in the same package: port `factory.go` initialize, temp copy, hot-swap, delayed close, `AllowMissing`, singleton `map[string]*BIN`.
- [x] 1.3 Unit-test MMDB singleton + hot-swap and BIN singleton + hot-swap (move/adapt `factory_test.go`). Package-level reset for tests.

## 2. Vendors

- [x] 2.1 IPinfo `New`: thin provider over one MMDB wrapper; map Lite/Core/Plus tags onto Record. Delete vendor-local factory map and `open`/`startDownload`.
- [x] 2.2 MaxMind `New`: same MMDB wrapper; keep GeoIP2 nested mapping. Delete vendor-local factory map and `open`/`startDownload`.
- [x] 2.3 IP2Location `New`: two BIN wrappers (geo + ASN). `Close` does not close the shared wrappers. Delete `factory.go`.

## 3. Docs and tests

- [x] 3.1 Update `knowledge/devdocs/core_geoblock_database_provider.md` (layers + `pkg/dbwrappers`; FromBytes gotcha stays).
- [x] 3.2 Provider tests: two `New` with the same config still Lookup after one `Close`.
- [x] 3.3 `go test ./...`
