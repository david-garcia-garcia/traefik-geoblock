## MODIFIED Requirements

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
