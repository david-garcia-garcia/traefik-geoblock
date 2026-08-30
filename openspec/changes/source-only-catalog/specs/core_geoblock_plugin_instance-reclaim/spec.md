## MODIFIED Requirements

### Requirement: New still binds wrappers to this context
When `mode` is `enrich` or `enrichandblock`, `NewCore` SHALL open enabled catalog sources on the incarnation lifetime so format wrappers stay bound even when the plugin incarnation is reused. When `mode` is `disabled` or `block`, `NewCore` MUST NOT open catalog sources.

#### Scenario: Reused plugin still binds wrappers
- **WHEN** `mode` is `enrichandblock` and a plugin incarnation is reused for a later `New` with a new live context
- **THEN** lookups on the returned handler still succeed
- **AND** that context is a holder on the format wrapper

#### Scenario: Block incarnation has no provider
- **WHEN** `mode` is `block` and `New` constructs or reuses a plugin incarnation
- **THEN** that incarnation has no open catalog sources
