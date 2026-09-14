## Purpose

CIDR allow and deny lists match a client address only when the written CIDR and the lookup IP belong to the same address family, so an IPv6 office prefix cannot silently allow or block an unrelated IPv4.

## Requirements

### Requirement: A CIDR matches only its address family
A CIDR stored for `allowedIPBlocks` or `blockedIPBlocks` (including CIDRs loaded from the matching directory lists) SHALL match a lookup IP only when both belong to the same address family. Family of a CIDR SHALL be the family of the parsed network address. Family of a lookup SHALL use the same classification as insert, from the `net.IP` already parsed for that hop. An IPv4-mapped IPv6 address (`::ffff:a.b.c.d`) SHALL be IPv4. The match owner SHALL be the CIDR lookup helper (`AddCIDR` / `IsContained`). The plugin MUST NOT re-classify family from hop headers or from MMDB/BIN.

#### Scenario: IPv4 slash-32 does not match colliding IPv6
- **WHEN** the helper contains `1.2.3.4/32`
- **AND** the lookup IP is `102:304::1`
- **THEN** `IsContained` reports not contained

#### Scenario: IPv4 slash-32 still matches IPv4
- **WHEN** the helper contains `1.2.3.4/32`
- **AND** the lookup IP is `1.2.3.4`
- **THEN** `IsContained` reports contained with prefix length 32

#### Scenario: IPv6 slash-32 does not match colliding IPv4
- **WHEN** the helper contains `808:808::/32`
- **AND** the lookup IP is `8.8.8.8`
- **THEN** `IsContained` reports not contained

#### Scenario: IPv6 slash-32 still matches IPv6
- **WHEN** the helper contains `808:808::/32`
- **AND** the lookup IP is `808:808::1`
- **THEN** `IsContained` reports contained with prefix length 32

#### Scenario: IPv4-mapped lookup follows IPv4
- **WHEN** the helper contains `1.2.3.4/32`
- **AND** the lookup IP is `::ffff:1.2.3.4`
- **THEN** `IsContained` reports contained with prefix length 32

### Requirement: Longest-prefix and catch-all stay inside one family
Longest-prefix matching SHALL consider only CIDRs of the lookup IP’s family. A family catch-all SHALL match only that family. Inserting `0.0.0.0/0` MUST NOT make an IPv6 lookup contained. Inserting `::/0` MUST NOT make an IPv4 lookup contained. A same-family `/0` SHALL still be contained with prefix length 0. `IsContained` SHALL keep returning contained plus prefix length; the return shape MUST NOT change.

#### Scenario: IPv4 catch-all does not match IPv6
- **WHEN** the helper contains `0.0.0.0/0`
- **AND** the lookup IP is `2001:db8::1`
- **THEN** `IsContained` reports not contained

#### Scenario: IPv6 catch-all does not match IPv4
- **WHEN** the helper contains `::/0`
- **AND** the lookup IP is `8.8.8.8`
- **THEN** `IsContained` reports not contained

#### Scenario: Same-family catch-all still matches
- **WHEN** the helper contains `0.0.0.0/0` and `::/0`
- **AND** the lookup IP is `8.8.8.8`
- **THEN** `IsContained` reports contained with prefix length 0

#### Scenario: Longest-prefix stays inside IPv4
- **WHEN** the helper contains `10.0.0.0/8` and `10.1.0.0/16`
- **AND** the lookup IP is `10.1.2.3`
- **THEN** `IsContained` reports contained with prefix length 16

### Requirement: An IPv6 allow-list does not exempt colliding IPv4
When `blockedCountries` would deny an IPv4 client, an `allowedIPBlocks` CIDR written for IPv6 MUST NOT allow that IPv4. The plugin SHALL use the helper’s contained result for CIDR allow and deny. A same-family IPv6 CIDR SHALL still allow that IPv6.

#### Scenario: Colliding IPv4 stays country-blocked
- **WHEN** `allowedIPBlocks` contains `808:808::/32`
- **AND** `blockedCountries` contains `US`
- **AND** the request IP is `8.8.8.8`
- **THEN** the request is blocked with reason `blocked_country`

#### Scenario: Same-family IPv6 CIDR still allows
- **WHEN** `allowedIPBlocks` contains `808:808::/32`
- **AND** `defaultAllow` is false
- **AND** the request IP is `808:808::1`
- **THEN** the request is allowed with reason `allowed_ip_block`
