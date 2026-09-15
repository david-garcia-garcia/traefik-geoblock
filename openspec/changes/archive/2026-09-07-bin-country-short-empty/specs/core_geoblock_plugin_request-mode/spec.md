## MODIFIED Requirements

### Requirement: Country header is the write/read bridge
When `mode` is not `disabled`, `countryHeader` SHALL be a request header name. Empty `countryHeader` SHALL default to `X-IPCountry`. Lookup (`enrich` or `enrichandblock`) SHALL write the ISO country, `PRIVATE` for a private or loopback hop, or `XX` for a public hop the enabled sources returned no country for, to that header. A BIN source that answers IP2Location `-` for `country_short` SHALL count as no country after merge. `XX` SHALL NOT mark the country written, so a later hop that resolves still wins. The block stage (`block` or `enrichandblock`) SHALL read that same header for country allow/block. Country rules MUST NOT take the lookup `Record` country directly. A `requestHeaderEnrich` mapping whose key is `country` and whose header name is not `countryHeader` SHALL also be written. Plugin creation MUST NOT fail because more than one header maps to `country`. When a request is handled by a `mode` `enrich` hop and then a `mode` `block` hop that share the same `countryHeader`, the block hop SHALL allow or deny using the country the enrich hop wrote.

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
