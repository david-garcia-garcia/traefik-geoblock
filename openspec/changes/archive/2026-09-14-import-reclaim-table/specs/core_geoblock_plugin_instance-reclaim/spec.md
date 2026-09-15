## MODIFIED Requirements

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
