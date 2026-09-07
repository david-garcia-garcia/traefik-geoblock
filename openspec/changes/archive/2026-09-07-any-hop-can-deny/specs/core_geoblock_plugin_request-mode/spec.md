## MODIFIED Requirements

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
