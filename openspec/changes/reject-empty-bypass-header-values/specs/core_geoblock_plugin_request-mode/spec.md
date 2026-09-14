## ADDED Requirements

### Requirement: Empty bypassHeaders values cannot skip blocking
When `mode` is not `disabled`, plugin creation SHALL fail if any `bypassHeaders` value is empty after trimming whitespace. A `bypassHeaders` map with no entries SHALL still succeed. When `mode` is `disabled`, plugin creation SHALL NOT fail because a `bypassHeaders` value is empty.

A `bypassHeaders` match SHALL skip the block stage only when the named header is present on the request and its value equals the configured value. An omitted header MUST NOT match an empty configured value. A leftover empty map entry MUST NOT write `pass:bypass_header` or skip blocking for a request that would otherwise be blocked. Non-empty configured values SHALL keep today's match behavior.

#### Scenario: Empty bypass value fails plugin creation
- **WHEN** the plugin is created with `mode` `enrichandblock` and `bypassHeaders` maps `X-Bypass` to `""`
- **THEN** plugin creation fails

#### Scenario: Whitespace-only bypass value fails plugin creation
- **WHEN** the plugin is created with `mode` `enrichandblock` and `bypassHeaders` maps `X-Bypass` to `"   "`
- **THEN** plugin creation fails

#### Scenario: Empty bypass map is allowed
- **WHEN** the plugin is created with `mode` `enrichandblock` and `bypassHeaders` has no entries
- **THEN** plugin creation succeeds

#### Scenario: Disabled mode does not reject empty bypass values
- **WHEN** the plugin is created with `mode` `disabled` and `bypassHeaders` maps `X-Bypass` to `""`
- **THEN** plugin creation succeeds

#### Scenario: Leftover empty bypass value does not skip a blocked country
- **WHEN** a loaded plugin has `bypassHeaders` mapping `X-Bypass` to `""`, `blockedCountries` includes `US`, and `defaultAllow` is false
- **AND** the request has a US client IP and omits `X-Bypass`
- **THEN** the request is blocked
- **AND** the decision header is not `pass:bypass_header`

#### Scenario: Non-empty bypass still skips when the header matches
- **WHEN** `bypassHeaders` maps `X-Bypass` to a non-empty secret and `blockedCountries` includes `US`
- **AND** the request has that header equal to the secret and a US client IP
- **THEN** the request is not country-blocked
- **AND** the decision header is `pass:bypass_header`
