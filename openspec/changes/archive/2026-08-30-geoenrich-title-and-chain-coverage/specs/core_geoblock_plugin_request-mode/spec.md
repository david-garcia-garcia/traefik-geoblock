## MODIFIED Requirements

### Requirement: Country header is the write/read bridge
When `mode` is not `disabled`, `countryHeader` SHALL be a request header name. Empty `countryHeader` SHALL default to `X-IPCountry`. Lookup (`enrich` or `enrichandblock`) SHALL write the ISO country or `PRIVATE` to that header. The block stage (`block` or `enrichandblock`) SHALL read that same header for country allow/block. Country rules MUST NOT take the lookup `Record` country directly. A `requestHeaderEnrich` mapping whose key is `country` and whose header name is not `countryHeader` SHALL also be written. Plugin creation MUST NOT fail because more than one header maps to `country`. When a request is handled by a `mode` `enrich` hop and then a `mode` `block` hop that share the same `countryHeader`, the block hop SHALL allow or deny using the country the enrich hop wrote.

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
