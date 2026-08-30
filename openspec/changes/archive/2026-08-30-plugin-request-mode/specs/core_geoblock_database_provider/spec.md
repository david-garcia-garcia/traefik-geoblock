## MODIFIED Requirements

### Requirement: Plugin looks up country through a provider
When `mode` is `enrich` or `enrichandblock`, the plugin SHALL obtain the country for an IP from a DatabaseProvider and SHALL write it to `countryHeader`. It MUST NOT call a vendor SDK type from the request path. The block stage SHALL use the `countryHeader` value for country allow/block, not the lookup `Record` directly. A failed lookup SHALL surface as an error to the existing `banIfError` / allow-on-error behavior. When `mode` is `block` or `disabled`, the plugin MUST NOT look up through a DatabaseProvider.

#### Scenario: Public IP lookup
- **WHEN** `mode` is `enrichandblock` and the plugin checks a public IP
- **THEN** it asks the provider for a country code
- **AND** it writes that code to `countryHeader`
- **AND** country allow/block reads `countryHeader`

#### Scenario: Lookup error
- **WHEN** the provider returns an error for an IP
- **THEN** the plugin applies the existing `banIfError` rule for that IP

#### Scenario: Block mode has no lookup
- **WHEN** `mode` is `block`
- **THEN** the plugin does not call DatabaseProvider Lookup
