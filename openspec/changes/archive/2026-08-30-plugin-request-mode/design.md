## Context

One enabled middleware always opens the geo database (`NewCore` → `bindDatabase`) and decides country from `Record.Country` in the same loop as header write. `countryHeader` is deprecated write-only. Operators copy `databaseSources` onto every route middleware (`docker-compose.yml`). Traefik `New` is per router; reclaim shares a Plugin only for the same name+config hash.

## Goals / Non-Goals

**Goals:**

- Config `mode` replaces `enabled`.
- Required `countryHeader` as the enrich/block bridge.
- Two stages in one `ServeHTTP`. Country rules read the header.
- Open DatabaseProvider only for `enrich` and `enrichandblock`.
- README: CheckAll country narrowing and existing alternates.

**Non-Goals:**

- Two Traefik plugins.
- Changing Traefik’s per-router `New`.
- A leftover `enabled` alias.
- An invented default country header name.
- Country-checking every hop after the header bridge.

## Decisions

- `mode` values are `disabled` | `enrich` | `block` | `enrichandblock`. Empty is `disabled`.
- Lookup writes `countryHeader` (and `requestHeaderEnrich`). Block reads `countryHeader` then CIDR / private / country maps.
- `block` does not call `setPrivateGeoHeaders`.
- `Prepare` skips catalog defaults, `IPHeaders`, and `countryHeader` when `disabled`. `block` validates `IPHeaders` and `countryHeader`, not catalog.
- `PRIVATE` on the header follows `allowPrivate`. Missing / empty / `null` uses `banIfError`.
- Operator puts enrich before block. Wrong order → missing header → `banIfError`.
- Remaining assumed explore rows stay assumed.

## Risks / Trade-offs

- **BREAKING:** every `enabled: true` must become `mode: enrichandblock` plus a `countryHeader`.
- CheckAll no longer country-blocks a later public proxy IP. CIDR and `ipHeaderStrategy` remain.
- `full` was renamed `enrichandblock` so the field is self-describing.

## Migration Plan

- README table: `mode` and required `countryHeader`.
- Compose labels: `mode=enrichandblock` and a `countryHeader` on every whoami that used `enabled=true`.
- Add a compose pair that shows shared enrich + per-route block if a PathPrefix can carry two middlewares.
