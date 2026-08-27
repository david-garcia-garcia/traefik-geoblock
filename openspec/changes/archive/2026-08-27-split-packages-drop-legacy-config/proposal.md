## Why

Deprecated response headers, a redundant pass/block request header, and file-based banned-request logging still sit beside the decision header operators should use. Helper and IP2Location code share the plugin package, which blocks a second geo database later.

Operators also want IPinfo Lite as a selectable geo database (country + ASN in one MMDB) with a committed snapshot so `New` works without a download token.

## What Changes

- **BREAKING**: Remove `remediationHeadersCustomName` (deprecated response header).
- **BREAKING**: Remove `logStatusHeader`. Pass/block+reason stays on `logStatusDetailHeader`.
- **BREAKING**: Remove `logBannedRequests`, `logPath`, `fileLogBufferSizeBytes`, `fileLogBufferTimeoutSeconds`. No file logger. No dedicated blocked-request info log. Remaining logs use the stdout slog logger (`logLevel` / `logFormat`).
- Split helpers into `pkg/` subpackages. Root package stays the Traefik plugin (`New` / `CreateConfig`).
- Add a DatabaseProvider abstraction (init, lookup, auto-update). Select it with `databaseProvider` (`ip2location` default, or `ipinfo`). IP2Location settings use `ip2location_*` keys. IPinfo Lite settings use `ipinfo_*` keys.
- Ship IPinfo Lite: bundled `ipinfo_lite.mmdb` as seed; token-only auto-update of the official MMDB URL; enrich every Lite column that has a value.

## Capabilities

### New Capabilities

- `core_geoblock_observability_decision-header`: Request decision observability is only `logStatusDetailHeader`. Removed header and file-log fields are gone.
- `core_geoblock_database_ipinfo-lite`: IPinfo Lite MMDB provider — bundled seed, token download, country/ASN enrich map. Not IP2Location `file=`.

### Modified Capabilities

- `core_geoblock_database_provider`: Plugin talks to a DatabaseProvider for open, country lookup, and auto-update. Implemented values are `ip2location` and `ipinfo`. Empty still defaults to `ip2location`. Unknown still fails. MaxMind is not implemented.

Token `file=` behavior (`core_geoblock_database_token-download-file`) does not change.

## Impact

- Traefik middleware config: operators must drop the removed keys. Observe blocks via `logStatusDetailHeader` + access logs.
- Operators who want IPinfo set `databaseProvider: ipinfo` and optional `ipinfo_*` keys. Default remains IP2Location.
- `plugin.go`, `logging.go`, `writer.go`, database/auto-update/file/IP helper files and tests.
- `README.md`, `docker-compose.yml`, `scripts/integration-tests.Tests.ps1`.
- New import paths under `github.com/david-garcia-garcia/traefik-geoblock/pkg/…`. Yaegi loads the root package and can import those subpackages.
- Bundled `ipinfo_lite.mmdb` (CC-BY-SA 4.0). Committed `vendor/` includes the MMDB reader Yaegi needs.
