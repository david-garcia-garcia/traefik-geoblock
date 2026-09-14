## Purpose

Reuses one plugin incarnation per Traefik middleware name and normalized config hash, so `New` does not rebuild the plugin for every router or same-config reload, while each returned handler still forwards to that router's next chain.

## Requirements

### Requirement: Same name and config share one plugin incarnation
When plugin `New` is invoked more than once with the same middleware name and the same normalized configuration, the plugin SHALL construct the plugin incarnation once and reuse it on the **plugin-root** reclaim table (`New(Config)` held at the Traefik constructor package). Each `New` MUST still return a handler whose next chain is the `next` passed to that `New`. Two `New` calls that differ in middleware name or in normalized configuration MUST NOT share an incarnation. `Open` SHALL pass `Hooks` whose `Close` cancels that Plugin’s `life`. Sleep and Wake SHALL be nil (the Plugin owns no keep-current ticker). Tests that tear down plugin incarnations SHALL Reset that plugin-root table; `dbwrappers.Reset` MUST NOT be the only teardown for `plugin:` keys.

#### Scenario: Two routers one incarnation
- **WHEN** `New` is invoked twice with the same middleware name and the same configuration
- **AND** each call receives a different next handler
- **THEN** the second `New` does not construct a second plugin incarnation
- **AND** a request through the first returned handler reaches the first next handler
- **AND** a request through the second returned handler reaches the second next handler

#### Scenario: Plugin Close cancels wrapper holders
- **WHEN** a plugin incarnation is Closed
- **THEN** that Plugin’s `life` context is canceled
- **AND** format wrappers bound on `life` drop that holder

#### Scenario: Name miss
- **WHEN** `New` is invoked twice with the same configuration and different middleware names
- **THEN** each call constructs its own plugin incarnation

#### Scenario: Config miss
- **WHEN** `New` is invoked twice with the same middleware name and different allowed-country lists
- **THEN** each call constructs its own plugin incarnation

### Requirement: New still binds wrappers to this context
When `mode` is `enrich` or `enrichandblock`, `NewCore` SHALL open enabled catalog sources on the incarnation lifetime so format wrappers stay bound even when the plugin incarnation is reused. When `mode` is `disabled` or `block`, `NewCore` MUST NOT open catalog sources.

#### Scenario: Reused plugin still binds wrappers
- **WHEN** `mode` is `enrichandblock` and a plugin incarnation is reused for a later `New` with a new live context
- **THEN** lookups on the returned handler still succeed
- **AND** that context is a holder on the format wrapper

#### Scenario: Block incarnation has no catalog sources
- **WHEN** `mode` is `block` and `New` constructs or reuses a plugin incarnation
- **THEN** that incarnation has no open catalog sources

### Requirement: Same name and config reclaim across reload
When every bound `New` context for a name+config key is cancelled and a later `New` uses the same name and configuration with a new live context before grace ends, the plugin MUST reuse the same incarnation and MUST NOT construct a new one.

#### Scenario: Same-hash New after generation cancel
- **WHEN** a plugin incarnation exists for name N and configuration C
- **AND** Traefik cancels those `New` contexts and calls `New` again with N and C before grace ends
- **THEN** a second plugin incarnation is not constructed
- **AND** the returned handler still applies C's allow/block rules

### Requirement: Unreclaimed incarnation is dropped after grace
When no live `New` context remains for a name+config key and grace elapses without a same-key `New`, the incarnation SHALL leave the plugin-root table. A later `New` with that name and configuration SHALL construct a new incarnation.

#### Scenario: Config no longer used
- **WHEN** the last `New` context for name N and configuration C is Done
- **AND** no `New` with N and C occurs during grace
- **THEN** a later `New` with N and C constructs a new plugin incarnation
