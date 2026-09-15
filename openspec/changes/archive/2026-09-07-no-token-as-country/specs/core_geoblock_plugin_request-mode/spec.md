## MODIFIED Requirements

### Requirement: Country header is the write/read bridge
When `mode` is not `disabled`, `countryHeader` SHALL be a request header name. Empty `countryHeader` SHALL default to `X-IPCountry`. Lookup (`enrich` or `enrichandblock`) SHALL write the ISO country, `PRIVATE` for a private or loopback hop, or `XX` for a public hop the enabled sources returned no country for, to that header. **Those values are exhaustive: an IP header value the plugin cannot parse, and an address whose lookup returns an error, SHALL NOT reach that header, and SHALL enrich as `XX`.** A BIN source that answers IP2Location `-` for `country_short` SHALL count as no country after merge. `XX` SHALL NOT mark the country written, so a later hop that resolves still wins. The block stage (`block` or `enrichandblock`) SHALL read that same header for country allow/block. Country rules MUST NOT take the lookup `Record` country directly. A `requestHeaderEnrich` mapping whose key is `country` and whose header name is not `countryHeader` SHALL also be written. Plugin creation MUST NOT fail because more than one header maps to `country`. When a request is handled by a `mode` `enrich` hop and then a `mode` `block` hop that share the same `countryHeader`, the block hop SHALL allow or deny using the country the enrich hop wrote.

#### Scenario: An unparseable IP header value is not a country
- **WHEN** `mode` is `enrich` and `X-Forwarded-For` is `Norway`
- **THEN** `countryHeader` is `XX`

#### Scenario: A later hop that resolves wins over an unparseable one
- **WHEN** `mode` is `enrich` and `X-Forwarded-For` is `DE, 8.8.8.8`
- **AND** `8.8.8.8` looks up as `US`
- **THEN** `countryHeader` is `US`
