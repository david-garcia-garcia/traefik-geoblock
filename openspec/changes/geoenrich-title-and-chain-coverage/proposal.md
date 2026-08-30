## Why

The README title still names only Geoblock. Operators already split lookup (`enrich`) from allow/block (`block`) on two Traefik middlewares, and they can run `block` with no prior lookup. Neither setup is in the compose/Pester harness. Package tests are not that proof.

## What Changes

- README H1 is Traefik Geoblock and Geoenrich.
- Compose + Pester: one route chains `mode` `enrich` then `mode` `block` on a shared `countryHeader`.
- Compose + Pester: one route is `mode` `block` only, with no prior enrich, so a missing `countryHeader` follows `banIfError`.
- Request-mode spec adds the enrich-then-block hop scenario. Missing-header `banIfError` is already specified.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_plugin_request-mode`: A request that passes `mode` `enrich` then `mode` `block` on the same `countryHeader` SHALL be allowed or blocked from the country the enrich hop wrote.

## Impact

- `README.md` — H1 title only.
- `docker-compose.yml` — `/enrichthenblock` and `/blockonly`.
- `scripts/integration-tests.Tests.ps1` — two Pester Contexts.
- `knowledge/devdocs/core_geoblock_test-harness.md` — PathPrefixes in How to use.
