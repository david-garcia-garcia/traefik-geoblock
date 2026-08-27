## 1. Helper packages

- [x] 1.1 Move file search/copy into `pkg/fileutils` and point callers at it.
- [x] 1.2 Move BIN version/date helpers into `pkg/dbutils`.
- [x] 1.3 Move CIDR radix + directory monitor into `pkg/iplookup`.

## 2. Database provider

- [x] 2.1 Add `pkg/dbprovider` with `Provider` (`LookupCountry`, `Close`) and the config the plugin passes in.
- [x] 2.2 Move factory, wrapper, and auto-update into `pkg/ip2location` implementing `Provider`. Keep token `file=` behavior.
- [x] 2.3 Wire `New` to the IP2Location provider. Plugin `Lookup` uses `LookupCountry` only.

## 3. Drop legacy logging and headers

- [x] 3.1 Remove `logStatusHeader`, `logBannedRequests`, `logPath`, `fileLogBufferSizeBytes`, `fileLogBufferTimeoutSeconds`, `remediationHeadersCustomName` from Config and Plugin.
- [x] 3.2 `setLogHeaders` writes only `logStatusDetailHeader`. Delete `writer.go` and file-logger path in `createLogger`.
- [x] 3.3 Stop the dedicated blocked-request info log.

## 4. Tests and docs

- [x] 4.1 Update unit tests and imports for moved types and removed fields.
- [x] 4.2 Update README, docker-compose, and integration tests (`X-Geoblock-Decision` only).
- [x] 4.3 Run `go test ./...`.

## 5. IPinfo Lite provider

- [x] 5.1 Add `pkg/ipinfo` implementing `dbprovider.Provider` (Lite MMDB lookup + token auto-update). Do not use IP2Location `file=`.
- [x] 5.2 Wire `databaseProvider: ipinfo` in `openDatabaseProvider`. Pass only `ipinfo_*` keys. Empty / `ip2location` stays IP2Location. Unknown still fails `New`.
- [x] 5.3 Empty `ipinfo_databaseFilePath` resolves bundled `ipinfo_lite.mmdb`. Auto-update without dir fails `New`. Auto-update without token logs and keeps the seed.
- [x] 5.4 Map Lite fields: `country_code` → allow/block country; enrich `country_name`, `continent`, `continent_code`, `isp` (`as_name`), `domain` (`as_domain`), `asn` (`AS` prefix). Empty enrich fields write `null`.
- [x] 5.5 Vendor `oschwald/maxminddb-golang` v1. Apply `scripts/apply-oschwald-yaegi-patch.ps1` (no mmap / no `x/sys`). Keep `ipinfo_lite.mmdb`. Credit IPinfo (CC-BY-SA 4.0) in README.
- [x] 5.6 Unit tests for lookup, empty path, auto-update-without-token, unknown provider. Compose `/ipinfo` + Pester Lite enrich headers.
- [x] 5.7 Run `go test ./...`. `Test-Integration.ps1` `/ipinfo` is a before-merge check, not a propose blocker.
