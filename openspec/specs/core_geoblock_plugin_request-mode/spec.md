## Purpose

Splits GeoIP lookup (enrichment) from country allow/block so one shared enrich middleware can own the database catalog and tokens, and per-route block middlewares can decide from a required country request header.

## Requirements

### Requirement: Mode replaces enabled
Traefik Config SHALL expose `mode` as `disabled`, `enrich`, `block`, or `enrichandblock`. Config SHALL NOT expose `enabled`. Empty `mode` SHALL be `enrichandblock`. An unknown `mode` SHALL fail plugin creation.

#### Scenario: Empty mode is enrichandblock
- **WHEN** the plugin is created with empty `mode`
- **THEN** plugin creation behaves as `mode` `enrichandblock` (lookup and allow/block)
- **AND** plugin creation opens enabled catalog sources

#### Scenario: Unknown mode fails
- **WHEN** the plugin is created with `mode` set to a value other than `disabled`, `enrich`, `block`, or `enrichandblock`
- **THEN** plugin creation fails

#### Scenario: No enabled field
- **WHEN** the operator sets `enabled` on the plugin Config
- **THEN** that field is not part of Config (Yaegi does not decode it onto the plugin)

### Requirement: Country header is the write/read bridge
When `mode` is not `disabled`, `countryHeader` SHALL be a request header name. Empty `countryHeader` SHALL default to `X-IPCountry`. Lookup (`enrich` or `enrichandblock`) SHALL write the ISO country, `PRIVATE` for a private or loopback hop, or `XX` for a public hop the enabled sources returned no country for, to that header. A BIN source that answers IP2Location `-` for `country_short` SHALL count as no country after merge. `XX` SHALL NOT mark the country written, so a later hop that resolves still wins. The block stage (`block` or `enrichandblock`) SHALL read that same header for country allow/block. Country rules MUST NOT take the lookup `Record` country directly. A `requestHeaderEnrich` mapping whose key is `country` and whose header name is not `countryHeader` SHALL also be written. Plugin creation MUST NOT fail because more than one header maps to `country`. When a request is handled by a `mode` `enrich` hop and then a `mode` `block` hop that share the same `countryHeader`, the block hop SHALL allow or deny using the country the enrich hop wrote.

#### Scenario: Enrich writes countryHeader
- **WHEN** `mode` is `enrich` and `countryHeader` is `X-IPCountry`
- **AND** the first public IP looks up as `US`
- **THEN** the request header `X-IPCountry` is `US`
- **AND** the request is not country-blocked by this instance

#### Scenario: Block reads countryHeader
- **WHEN** `mode` is `block`, `countryHeader` is `X-IPCountry`, and `blockedCountries` includes `US`
- **AND** the request already has `X-IPCountry: US`
- **THEN** the request is blocked
- **AND** plugin creation did not open catalog sources

#### Scenario: Empty countryHeader defaults
- **WHEN** `mode` is `enrichandblock` and `countryHeader` is empty
- **THEN** plugin creation succeeds
- **AND** `countryHeader` is `X-IPCountry`

#### Scenario: Extra country enrich header is written
- **WHEN** `mode` is `enrich`, `countryHeader` is `X-IPCountry`, and `requestHeaderEnrich` maps `X-Geo-Country` to `country`
- **AND** the first public IP looks up as `US`
- **THEN** plugin creation succeeds
- **AND** the request header `X-IPCountry` is `US`
- **AND** the request header `X-Geo-Country` is `US`

#### Scenario: Enrich hop then block hop
- **WHEN** a request is handled by `mode` `enrich` then `mode` `block` that share `countryHeader` `X-IPCountry`
- **AND** the enrich hop writes `US`
- **AND** the block hop `blockedCountries` includes `US`
- **THEN** the request is blocked

#### Scenario: BIN-only miss writes XX
- **WHEN** `mode` is `enrichandblock` and the only enabled country source is BIN
- **AND** the selected hop is a public IP whose BIN `country_short` is `-`
- **THEN** `countryHeader` is `XX`
- **AND** the country is not marked written

#### Scenario: XX covers a BIN miss in allowedCountries
- **WHEN** `mode` is `enrichandblock`, `defaultAllow` is false, and `allowedCountries` includes `XX`
- **AND** the only enabled country source is BIN
- **AND** the selected hop is a public IP whose BIN `country_short` is `-`
- **THEN** the request is allowed with reason `allowed_country`

### Requirement: Catalog sources open only for lookup modes
Plugin creation SHALL open enabled `databaseSources` rows only when `mode` is `enrich` or `enrichandblock`. When `mode` is `disabled` or `block`, creation MUST NOT open catalog sources, MUST NOT insert default catalog rows, and MUST NOT start auto-update.

#### Scenario: Block does not open the database
- **WHEN** the plugin is created with `mode` `block` and a valid `countryHeader`
- **THEN** no catalog source is opened
- **AND** no default `databaseSources` row is inserted

#### Scenario: Enrich opens the database
- **WHEN** the plugin is created with `mode` `enrich` and a valid `countryHeader`
- **THEN** plugin creation opens the enabled catalog sources

### Requirement: Block stage still applies CIDR and private
When `mode` is `block` or `enrichandblock`, the plugin SHALL still extract IPs with `IPHeaders` / `ipHeaderStrategy` and SHALL apply `allowedIPBlocks`, `blockedIPBlocks`, and `allowPrivate`. A missing, empty, or `null` `countryHeader` value SHALL use `banIfError`. `PRIVATE` on `countryHeader` SHALL follow `allowPrivate`. Country allow/block SHALL use only the `countryHeader` value (first public written), not a later hop's looked-up country. `CheckAll` SHALL still apply CIDR and private per selected IP.

Every selected hop SHALL be able to deny: an earlier allowed hop MUST NOT suppress a later hop's denial, nor a later hop's `banIfError` ban. The pass reason on the decision header SHALL be the phase of the first allowing hop.

#### Scenario: Block CIDR without a database
- **WHEN** `mode` is `block` and `blockedIPBlocks` contains `8.8.8.8/32`
- **AND** the request IP is `8.8.8.8`
- **THEN** the request is blocked
- **AND** no catalog source was opened

#### Scenario: Missing country header uses banIfError
- **WHEN** `mode` is `block`, `banIfError` is true, and `countryHeader` is absent on the request
- **THEN** the request is blocked

#### Scenario: A blocked CIDR after an allowed hop still denies
- **WHEN** `ipHeaderStrategy` is `CheckAll`, `defaultAllow` is true, and `blockedIPBlocks` contains `1.1.1.0/24`
- **AND** the IP chain is `8.8.8.8, 1.1.1.1`
- **THEN** the request is blocked
- **AND** the decision header is `block:blocked_ip_block`

#### Scenario: An allowedIPBlocks hop does not exempt the hops after it
- **WHEN** `ipHeaderStrategy` is `CheckAll`, `allowedIPBlocks` contains `8.8.8.0/24`, `blockedCountries` contains `US`
- **AND** the IP chain is `8.8.8.8, 1.1.1.1` and `countryHeader` resolved to `US`
- **THEN** the request is blocked
- **AND** the decision header is `block:blocked_country`

#### Scenario: banIfError bans an unparseable later hop
- **WHEN** `mode` is `block`, `banIfError` is true, `countryHeader` is present and allowed
- **AND** the IP chain is `8.8.8.8, not-an-ip`
- **THEN** the request is blocked
- **AND** the decision header is `block:error`

### Requirement: Block must not overwrite inbound enrich headers
When `mode` is `block`, the plugin MUST NOT write `countryHeader` or `requestHeaderEnrich` values (`PRIVATE` / `null`) before reading the inbound country.

#### Scenario: Block preserves inbound country
- **WHEN** `mode` is `block` and the request has `X-IPCountry: DE`
- **THEN** after this middleware runs (if allowed), `X-IPCountry` is still `DE`
