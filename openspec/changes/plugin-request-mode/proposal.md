## Why

Enrichment (database URLs, tokens) is shared; country allow/block lists are per route. One enabled middleware always opens the geo database and decides country from the lookup record, so operators copy download config onto every route middleware.

## What Changes

- **BREAKING:** Remove Config `enabled`. Add `mode`: `disabled` | `enrich` | `block` | `enrichandblock`. Empty/`CreateConfig` is `disabled`. Unknown `mode` fails `Prepare`. No `enabled` alias. Existing `enabled: true` becomes `mode: enrichandblock`.
- **BREAKING:** `countryHeader` is required when `mode` is not `disabled`. It is the bridge: lookup writes the country there; the block stage reads it. No invented default name. Empty fails `Prepare`. A second `requestHeaderEnrich` header mapped to `country` that is not `countryHeader` fails `Prepare`.
- Two stages, one `ServeHTTP`: lookup (`enrich` | `enrichandblock`) writes headers; block (`block` | `enrichandblock`) reads `countryHeader` then applies CIDR / private / country lists. Country rules do not take the lookup `Record` directly.
- **MUST:** `bindDatabase` / `openDatabaseProvider` run only for `enrich` and `enrichandblock`. `disabled` and `block` MUST NOT open a DatabaseProvider, insert default catalog rows, or start auto-update. `block` may still load CIDR file monitors.
- **BREAKING (behavior):** Country allow/block uses the one `countryHeader` value (first public written). `CheckAll` still applies CIDR and private per IP. A later public proxy IP is no longer country-checked. README documents `ipHeaderStrategy`, CIDR blocks, and `ipHeaders` as the alternates.
- `block` MUST NOT write enrich headers (`setPrivateGeoHeaders` would replace inbound country with `PRIVATE`).
- README + compose: `mode`, required `countryHeader`, split enrich-then-block example.

## Capabilities

### New Capabilities

- `core_geoblock_plugin_request-mode`: Config `mode` and required `countryHeader` bridge; lookup vs block stages; when the DatabaseProvider opens.

### Modified Capabilities

- `core_geoblock_plugin_instance-reclaim`: `NewCore` opens the DatabaseProvider only for `enrich` and `enrichandblock`.
- `core_geoblock_database_provider`: Country for allow/block is the `countryHeader` value after lookup write, not `Record.Country` in the same call.

## Impact

- `pkg/geoblock/config.go` — `Mode` replaces `Enabled`; `Prepare` gates catalog / `countryHeader` / `IPHeaders` by mode.
- `pkg/geoblock/plugin.go` — `NewCore`, `bindDatabase`, `ServeHTTP`, `CheckAllowed` country from header.
- `plugin.go` — Yaegi `Config` alias (same struct).
- Tests, README, `docker-compose.yml`, usage packets (`core_geoblock_plugin_instance.md`, `core_geoblock_database_provider.md`).
