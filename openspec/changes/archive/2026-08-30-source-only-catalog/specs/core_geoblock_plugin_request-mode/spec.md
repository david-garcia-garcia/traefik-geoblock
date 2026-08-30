## MODIFIED Requirements

### Requirement: Mode replaces enabled
Traefik Config SHALL expose `mode` as `disabled`, `enrich`, `block`, or `enrichandblock`. Config SHALL NOT expose `enabled`. Empty `mode` SHALL be `disabled`. An unknown `mode` SHALL fail plugin creation.

#### Scenario: Empty mode is disabled
- **WHEN** the plugin is created with empty `mode`
- **THEN** requests pass through with no lookup and no allow/block
- **AND** plugin creation does not open catalog sources

#### Scenario: Unknown mode fails
- **WHEN** the plugin is created with `mode` set to a value other than `disabled`, `enrich`, `block`, or `enrichandblock`
- **THEN** plugin creation fails

#### Scenario: No enabled field
- **WHEN** the operator sets `enabled` on the plugin Config
- **THEN** that field is not part of Config (Yaegi does not decode it onto the plugin)

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
When `mode` is `block` or `enrichandblock`, the plugin SHALL still extract IPs with `IPHeaders` / `ipHeaderStrategy` and SHALL apply `allowedIPBlocks`, `blockedIPBlocks`, and `allowPrivate`. A missing, empty, or `null` `countryHeader` value SHALL use `banIfError`. `PRIVATE` on `countryHeader` SHALL follow `allowPrivate`. Country allow/block SHALL use only the `countryHeader` value (first public written), not a later hop’s looked-up country. `CheckAll` SHALL still apply CIDR and private per selected IP.

#### Scenario: Block CIDR without a database
- **WHEN** `mode` is `block` and `blockedIPBlocks` contains `8.8.8.8/32`
- **AND** the request IP is `8.8.8.8`
- **THEN** the request is blocked
- **AND** no catalog source was opened
