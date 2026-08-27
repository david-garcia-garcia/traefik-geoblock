## 1. Provider package

- [x] 1.1 Add `pkg/maxmind` DatabaseProvider: `FromBytes` open, GeoIP2 nested lookup (`country.iso_code`), factory singleton.
- [x] 1.2 Resolve seed: empty path → `GeoIP2-Country-Test.mmdb`; auto-update dir dated file wins.
- [x] 1.3 Official auto-update: parse `accountId:licenseKey`, permalink + Basic Auth + redirect, tar.gz → dated MMDB, hot-swap. Empty/invalid token keeps seed.
- [x] 1.4 Reject ASN-only / unknown `maxmind_databaseAutoUpdateCode`. Default `GeoLite2-Country`.

## 2. Plugin wiring

- [x] 2.1 Add `DatabaseProviderMaxMind` and `maxmind_*` Config keys. Wire `openDatabaseProvider`.
- [x] 2.2 Change `UnsupportedDatabaseProvider` to a name other than `maxmind`. Add `New` success with dummy seed.

## 3. Docs and compose

- [x] 3.1 README: third provider, dummy seed, token `accountId:licenseKey`, no P3TERX, GeoLite attribution if we mention GeoLite editions.
- [x] 3.2 Compose `/maxmind` + Pester using a dummy-fixture IP.
- [x] 3.3 Update `knowledge/devdocs/core_geoblock_database_provider.md` and plugin packages key files.

## 4. Tests

- [x] 4.1 Package tests: dummy lookup ISO, empty path, auto-update without token keeps seed, invalid token, ASN code fails, URL/token parse without network.
- [x] 4.2 `go test ./...`
