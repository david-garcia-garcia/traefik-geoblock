## MODIFIED Requirements

### Requirement: Country header is the write/read bridge
When `mode` is not `disabled`, `countryHeader` SHALL be a request header name. Empty `countryHeader` SHALL default to `X-IPCountry`. Lookup (`enrich` or `enrichandblock`) SHALL write the ISO country, `PRIVATE` for a private or loopback hop, or `XX` for a public hop the enabled sources returned no country for, to that header. `XX` SHALL NOT mark the country written, so a later hop that resolves still wins. The block stage (`block` or `enrichandblock`) SHALL read that same header for country allow/block. Country rules MUST NOT take the lookup `Record` country directly. A `requestHeaderEnrich` mapping whose key is `country` and whose header name is not `countryHeader` SHALL also be written. Plugin creation MUST NOT fail because more than one header maps to `country`.

#### Scenario: Unresolved public IP follows defaultAllow
- **WHEN** `mode` is `enrichandblock`, `allowPrivate` is true, and `defaultAllow` is false
- **AND** the only selected hop is a public IP that no enabled source resolves
- **THEN** `countryHeader` is `XX`
- **AND** the request is blocked with reason `default_allow`

#### Scenario: Unresolved public IP does not lock the country
- **WHEN** `mode` is `enrichandblock` and `ipHeaderStrategy` is `CheckAll`
- **AND** the first selected hop is a public IP no source resolves and a later hop looks up as `GB`
- **THEN** `countryHeader` is `GB`

#### Scenario: Private hop keeps PRIVATE
- **WHEN** `mode` is `enrichandblock` and every selected hop is private or loopback
- **THEN** `countryHeader` is `PRIVATE`
- **AND** the request follows `allowPrivate`
