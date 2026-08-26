## Why

Deprecated response headers, a redundant pass/block request header, and file-based banned-request logging still sit beside the decision header operators should use. Helper and IP2Location code share the plugin package, which blocks a second geo database later.

## What Changes

- **BREAKING**: Remove `remediationHeadersCustomName` (deprecated response header).
- **BREAKING**: Remove `logStatusHeader`. Pass/block+reason stays on `logStatusDetailHeader`.
- **BREAKING**: Remove `logBannedRequests`, `logPath`, `fileLogBufferSizeBytes`, `fileLogBufferTimeoutSeconds`. No file logger. No dedicated blocked-request info log. Remaining logs use the stdout slog logger (`logLevel` / `logFormat`).
- Split helpers into `pkg/` subpackages. Root package stays the Traefik plugin (`New` / `CreateConfig`).
- Add a DatabaseProvider abstraction (init, lookup, auto-update). Select it with `databaseProvider` (only `ip2location` now). IP2Location settings use `ip2location_*` keys.

## Capabilities

### New Capabilities

- `core_geoblock_observability_decision-header`: Request decision observability is only `logStatusDetailHeader`. Removed header and file-log fields are gone.
- `core_geoblock_database_provider`: Plugin talks to a DatabaseProvider for open, country lookup, and auto-update. Only IP2Location is implemented.

### Modified Capabilities

None. Token `file=` behavior (`core_geoblock_database_token-download-file`) does not change.

## Impact

- Traefik middleware config: operators must drop the removed keys. Observe blocks via `logStatusDetailHeader` + access logs.
- `plugin.go`, `logging.go`, `writer.go`, database/auto-update/file/IP helper files and tests.
- `README.md`, `docker-compose.yml`, `scripts/integration-tests.Tests.ps1`.
- New import paths under `github.com/david-garcia-garcia/traefik-geoblock/pkg/…`. Yaegi loads the root package and can import those subpackages.
