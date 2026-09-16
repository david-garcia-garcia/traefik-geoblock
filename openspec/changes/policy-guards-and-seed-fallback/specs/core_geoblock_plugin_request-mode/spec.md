## ADDED Requirements

### Requirement: More specific CIDR prefix wins
When both `allowedIPBlocks` and `blockedIPBlocks` contain the client hop, the plugin SHALL apply the longer prefix. Prefix length 0 is a match (`0.0.0.0/0` or `::/0`). A `/32` (or `/128`) block SHALL beat a `/0` allow. Equal lengths SHALL keep today's allow-before-block order. Families stay isolated (`std_go_iplookup_family-isolated`).

#### Scenario: Catch-all allow loses to a more specific block
- **WHEN** `mode` is `block`, `allowedIPBlocks` contains `0.0.0.0/0`, `blockedIPBlocks` contains `8.8.8.8/32`, and `defaultAllow` is false
- **AND** the request IP is `8.8.8.8`
- **THEN** the request is blocked
- **AND** the decision header is `block:blocked_ip_block`

### Requirement: Empty hop list uses banIfError
When `GetRemoteIPs` returns no hop, the block stage SHALL treat that as a lookup-class error: log a warning and apply `banIfError`. The plugin MUST NOT invent a client address and MUST NOT pass with `pass:none` while `banIfError` is true.

#### Scenario: No client IP blocks when banIfError
- **WHEN** `mode` is `enrichandblock`, `banIfError` is true, `defaultAllow` is false
- **AND** `GetRemoteIPs` returns no hop
- **THEN** the request is blocked
- **AND** the decision header is `block:error`

#### Scenario: No client IP passes when banIfError is false
- **WHEN** `mode` is `block`, `banIfError` is false
- **AND** `GetRemoteIPs` returns no hop
- **THEN** the request is not blocked by this error path

### Requirement: Country header mapped to a non-country enrich key warns
When `requestHeaderEnrich` maps `countryHeader` to a metadata key that is not `country`, plugin creation SHALL succeed and SHALL warn that that header is filled with a non-country value. The enrich mapping SHALL still win over the folded `country` mapping.

#### Scenario: City mapping on countryHeader warns and loads
- **WHEN** `mode` is `enrichandblock`, `countryHeader` is `X-IPCountry`, and `requestHeaderEnrich` maps `X-IPCountry` to `city`
- **THEN** plugin creation succeeds
- **AND** a warning says `X-IPCountry` is filled with a non-country value

### Requirement: Block-mode Lookup does not panic
When `mode` is `block`, `Lookup` and `CheckAllowed` SHALL return an error that no catalog is bound. They MUST NOT dereference a nil catalog. `ServeHTTP` SHALL still decide from `countryHeader` and CIDR lists only.

#### Scenario: Lookup on a block-mode plugin errors
- **WHEN** the plugin was created with `mode` `block`
- **AND** `Lookup` is called with `8.8.8.8`
- **THEN** `Lookup` returns an error
- **AND** the process does not panic
