# Requirement
IssueKey: 2026-09-15-update-seed-databases

## Problem
The three committed seed databases last landed with #60 (`2ff5cfd`, 2026-08-28). The ticket asks to replace those files with newer official builds of the same product/edition, keep the same filenames unless a vendor renamed that product, and leave plugin behavior unchanged except as required for the newer files to open.

## Current (code)
- `seeds/IP2LOCATION-LITE-DB1.IPV6.BIN` (10,349,586 bytes) is the shipped IP2Location LITE country IPv6 BIN. Reserved catalog `default_ip2location` binds that basename and the free LITE ZIP URL (`pkg/geoblock/config.go`). `tools/dbdownload/main.go` GETs `https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP`, extracts that BIN name, and refuses the write unless `1.1.1.1` → `US`. `plugin.go` `go:generate` runs that CLI onto `seeds/`.
- `seeds/ipinfo_lite.mmdb` (23,496,022 bytes) is the shipped IPinfo Lite snapshot. Reserved `default_ipinfo` is disabled and names that file (`pkg/geoblock/config.go`). Official full Lite MMDB download is token-gated (`knowledge/research/ext_ipinfo_lite-database/notes.md`). Tests pin lookups (`pkg/dbwrappers/ipinfo_test.go`: `8.8.8.8` US / AS15169 / Google LLC; `1.1.1.1` AU; `85.214.132.117` DE / Strato / AS6724).
- `seeds/GeoIP2-Country-Test.mmdb` (19,492 bytes) is MaxMind’s official dummy Country fixture, not a live GeoLite file (`openspec/specs/core_geoblock_database_maxmind-geolite/spec.md`). Reserved `default_maxmind` is disabled and names that file (`pkg/geoblock/config.go`). Official dummy lives under `maxmind/MaxMind-DB` `test-data` (`knowledge/research/ext_maxmind_geolite2-database/notes.md`). Tests pin dummy IPs `81.2.69.142` → GB and `175.16.199.1` → CN (`pkg/dbwrappers/geoip2_test.go`, `scripts/integration-tests.Tests.ps1`).
- Resolve opens `{TRAEFIK_PLUGIN_GEOBLOCK_PATH}/seeds/<defaultFile>` when no dated Latest and no operator `path` (`pkg/fileutils/fileutils.go`, `knowledge/devdocs/core_geoblock_database_source.md`). Package tests join those same three paths (`pkg/geoblock/plugin_test.go`).

## Desired
Replace the three committed seed bytes with newer official builds of the same product/edition. Keep the same seed filenames unless the vendor renamed that same product. Do not add a vendor, a public config key, or a new seed name. Do not change plugin behavior except as required for the newer files to open.

## Affected
- `seeds/IP2LOCATION-LITE-DB1.IPV6.BIN`
- `seeds/ipinfo_lite.mmdb`
- `seeds/GeoIP2-Country-Test.mmdb`
- Callers that open those paths: reserved catalog defaults (`pkg/geoblock/config.go`), `tools/dbdownload/main.go`, package and integration tests that look up pinned IPs.

## Out of scope
- New vendor, new public config, or new seed filenames (unless a vendor rename of the same product).
- Plugin behavior changes except open compatibility with the newer files.
- Replacing `default_geolite` or committing a live GeoLite2 MMDB (`pkg/geoblock/config.go` `DefaultGeoliteURL`).
- New download automation for IPinfo or the MaxMind dummy (ticket did not ask).
- Changing `tools/dbdownload` URL or verify rule unless the official LITE ZIP or BIN name moved.

## Unknowns
- Whether `GeoIP2-Country-Test.mmdb` on `maxmind/MaxMind-DB` `test-data` is newer than the committed 19,492-byte file.
- Whether this run has an IPinfo account token for `https://ipinfo.io/data/ipinfo_lite.mmdb` (required for the full official file).
- Whether a newer IP2Location LITE still maps `1.1.1.1` → `US` so `tools/dbdownload` verify succeeds.
- Whether a newer IPinfo Lite still matches the pinned org/ASN rows in `pkg/dbwrappers/ipinfo_test.go`.

## Tensions
- `tools/dbdownload/main.go` treats `1.1.1.1` as `US` (IP2Location verify). Policy and IPinfo tests treat `1.1.1.1` as AU (`pkg/geoblock/plugin_policy_test.go`, `pkg/dbwrappers/ipinfo_test.go`). A LITE refresh that follows dbdownload may still disagree with those AU comments.
- Ticket “newer official builds” for the MaxMind dummy: research says that file is a test fixture, not a periodically published GeoLite edition. A byte-identical result is possible.
- IPinfo official full Lite is token-gated; the committed seed is a full snapshot, not the 100-row public sample (`knowledge/research/ext_ipinfo_lite-database/notes.md`).
