## Context

`databaseProvider` already switches IP2Location and IPinfo. IPinfo reads MMDB with vendored oschwald v1 `FromBytes` (Yaegi-safe). GeoIP2 country is nested `country.iso_code`; IPinfo is flat `country_code`. Official MaxMind download is Basic Auth (`accountId:licenseKey`) to a permalink that returns `tar.gz`. Live GeoLite files must not be committed; the seed is MaxMind’s dummy `GeoIP2-Country-Test.mmdb`.

## Goals / Non-Goals

**Goals:**

- Third provider `maxmind` in `pkg/maxmind`.
- Dummy seed when path is empty.
- Official auto-update with the same key shape as IPinfo (`*AutoUpdate`, `*Dir`, `*Token`, `*Code`).
- Country allow/block uses `country.iso_code`.

**Non-Goals:**

- P3TERX or any unofficial download URL.
- Committing a live GeoLite MMDB.
- `allowedASN` / reopening #42.
- Adding `geoip2-golang` or oschwald v2.
- Changing IP2Location `file=` or IPinfo download.
- Proving official download in CI without a token.

## Decisions

- Package `pkg/maxmind` mirrors `pkg/ipinfo` (factory singleton, `FromBytes`, 24h ticker, dated files, lock file).
- Token is one field: `accountId:licenseKey`. Split on the first `:`. Invalid/empty token → log + keep seed.
- Download URL: `https://download.maxmind.com/geoip/databases/{code}/download?suffix=tar.gz` with Basic Auth. Follow redirects. Extract the `.mmdb` from the tar.gz.
- Default code `GeoLite2-Country`. Reject `GeoLite2-ASN` and unknown editions.
- Dummy seed filename `GeoIP2-Country-Test.mmdb` at module root. `/maxmind` compose uses dummy test IPs.
- Do not type-assert the provider from `plugin.go`.

## Risks / Trade-offs

- Dummy fixture IPs are not real-world (8.8.8.8 will miss). Compose and package tests must use documented dummy ranges.
- Official download untested without `.env` token (30/day cap, secrets).
- tar.gz extract must pick the `.mmdb` member and not follow unsafe paths.

## Migration Plan

- Default provider unchanged. Operators opt in with `databaseProvider: maxmind`.
- No config rename. Rollback: omit `maxmind` or leave default IP2Location.

## Open Questions

None. Explore rows taken: selector `maxmind`; leaf `core_geoblock_database_maxmind-geolite`.
