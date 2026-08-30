## Purpose

Splits GeoIP lookup (enrichment) from country allow/block so one shared enrich middleware can own the database catalog and tokens, and per-route block middlewares can decide from a required country request header.

## Requirements

### Requirement: Mode replaces enabled
Traefik Config SHALL expose `mode` as `disabled`, `enrich`, `block`, or `enrichandblock`. Config SHALL NOT expose `enabled`. Empty `mode` SHALL be `disabled`. An unknown `mode` SHALL fail plugin creation.

#### Scenario: Empty mode is disabled
- **WHEN** the plugin is created with empty `mode`
- **THEN** requests pass through with no lookup and no allow/block
- **AND** plugin creation does not open a DatabaseProvider

#### Scenario: Unknown mode fails
- **WHEN** the plugin is created with `mode` set to a value other than `disabled`, `enrich`, `block`, or `enrichandblock`
- **THEN** plugin creation fails

#### Scenario: No enabled field
- **WHEN** the operator sets `enabled` on the plugin Config
- **THEN** that field is not part of Config (Yaegi does not decode it onto the plugin)

### Requirement: Country header is the write/read bridge
When `mode` is not `disabled`, `countryHeader` SHALL be a request header name. Empty `countryHeader` SHALL default to `X-IPCountry`. Lookup (`enrich` or `enrichandblock`) SHALL write the ISO country or `PRIVATE` to that header. The block stage (`block` or `enrichandblock`) SHALL read that same header for country allow/block. Country rules MUST NOT take the lookup `Record` country directly. A `requestHeaderEnrich` mapping whose key is `country` and whose header name is not `countryHeader` SHALL fail plugin creation.

#### Scenario: Enrich writes countryHeader
- **WHEN** `mode` is `enrich` and `countryHeader` is `X-IPCountry`
- **AND** the first public IP looks up as `US`
- **THEN** the request header `X-IPCountry` is `US`
- **AND** the request is not country-blocked by this instance

#### Scenario: Block reads countryHeader
- **WHEN** `mode` is `block`, `countryHeader` is `X-IPCountry`, and `blockedCountries` includes `US`
- **AND** the request already has `X-IPCountry: US`
- **THEN** the request is blocked
- **AND** plugin creation did not open a DatabaseProvider

#### Scenario: Empty countryHeader defaults
- **WHEN** `mode` is `enrichandblock` and `countryHeader` is empty
- **THEN** plugin creation succeeds
- **AND** `countryHeader` is `X-IPCountry`

### Requirement: Provider opens only for lookup modes
Plugin creation SHALL call `openDatabaseProvider` only when `mode` is `enrich` or `enrichandblock`. When `mode` is `disabled` or `block`, creation MUST NOT open a DatabaseProvider, MUST NOT insert default catalog rows, and MUST NOT start auto-update.

#### Scenario: Block does not open the database
- **WHEN** the plugin is created with `mode` `block` and a valid `countryHeader`
- **THEN** no DatabaseProvider is opened
- **AND** no default `databaseSources` row is inserted

#### Scenario: Enrich opens the database
- **WHEN** the plugin is created with `mode` `enrich` and a valid `countryHeader`
- **THEN** plugin creation opens the DatabaseProvider for the selected `databaseProvider`

### Requirement: Block stage still applies CIDR and private
When `mode` is `block` or `enrichandblock`, the plugin SHALL still extract IPs with `IPHeaders` / `ipHeaderStrategy` and SHALL apply `allowedIPBlocks`, `blockedIPBlocks`, and `allowPrivate`. A missing, empty, or `null` `countryHeader` value SHALL use `banIfError`. `PRIVATE` on `countryHeader` SHALL follow `allowPrivate`. Country allow/block SHALL use only the `countryHeader` value (first public written), not a later hop’s looked-up country. `CheckAll` SHALL still apply CIDR and private per selected IP.

#### Scenario: Block CIDR without a database
- **WHEN** `mode` is `block` and `blockedIPBlocks` contains `8.8.8.8/32`
- **AND** the request IP is `8.8.8.8`
- **THEN** the request is blocked
- **AND** no DatabaseProvider was opened

#### Scenario: Missing country header uses banIfError
- **WHEN** `mode` is `block`, `banIfError` is true, and `countryHeader` is absent on the request
- **THEN** the request is blocked

### Requirement: Block must not overwrite inbound enrich headers
When `mode` is `block`, the plugin MUST NOT write `countryHeader` or `requestHeaderEnrich` values (`PRIVATE` / `null`) before reading the inbound country.

#### Scenario: Block preserves inbound country
- **WHEN** `mode` is `block` and the request has `X-IPCountry: DE`
- **THEN** after this middleware runs (if allowed), `X-IPCountry` is still `DE`
