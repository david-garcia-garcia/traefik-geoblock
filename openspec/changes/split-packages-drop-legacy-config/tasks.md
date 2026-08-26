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
