## Why

Operators who already download MaxMind GeoLite/GeoIP2 MMDBs cannot select that vendor today. `databaseProvider: maxmind` fails plugin creation. The provider switch was built so a later vendor can be added without changing allow/block rules.

## What Changes

- Add `databaseProvider: maxmind` as a third DatabaseProvider (`pkg/maxmind`).
- Prefixed keys: `maxmind_databaseFilePath`, `maxmind_databaseAutoUpdate`, `maxmind_databaseAutoUpdateDir`, `maxmind_databaseAutoUpdateToken`, `maxmind_databaseAutoUpdateCode`.
- Empty path opens committed official dummy `GeoIP2-Country-Test.mmdb` (not a live GeoLite file, not P3TERX).
- Lookup uses GeoIP2 nested `country.iso_code` (not IPinfo flat `country_code`).
- Official auto-update: permalink + HTTP Basic Auth. Token value is `accountId:licenseKey`. Default edition `GeoLite2-Country`. No IP2Location `file=`.
- Drop “MaxMind SHALL NOT be implemented” from the provider spec.

## Capabilities

### New Capabilities

- `core_geoblock_database_maxmind-geolite`: MaxMind / GeoLite2 MMDB provider — dummy seed, GeoIP2 country map, official token auto-update.

### Modified Capabilities

- `core_geoblock_database_provider`: Implemented values include `maxmind`. Unknown still fails. MaxMind is implemented.

Token `file=` (`core_geoblock_database_token-download-file`) does not change. IPinfo Lite does not change.

## Impact

- Operators set `databaseProvider: maxmind` and optional `maxmind_*` keys. Default stays IP2Location.
- `plugin.go` `openDatabaseProvider`, Config, `plugin_config_test.go`.
- New `pkg/maxmind`.
- `README.md`, `docker-compose.yml` `/maxmind`, Pester. Dummy test IPs only on that route.
- Bundled `GeoIP2-Country-Test.mmdb` from maxmind/MaxMind-DB test-data.
