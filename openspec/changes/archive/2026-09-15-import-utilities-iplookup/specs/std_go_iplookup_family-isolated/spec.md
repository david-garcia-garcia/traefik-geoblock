## Purpose

This plugin looks up CIDR allow/block membership with a family-isolated helper from vendored traefik-middleware-utilities, so an IPv4 prefix cannot match an IPv6 hop and the reverse.

## ADDED Requirements

### Requirement: Helper is the vendored utilities package
This module SHALL import `github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup` from the version pinned in `go.mod`, with sources under `vendor/`. It SHALL NOT keep a first-party `pkg/iplookup` package. Library tests stay in the utilities module (`go mod vendor` omits `*_test.go`).

#### Scenario: Production CIDR lookup uses utilities iplookup
- **WHEN** production CIDR allow/block call sites are listed
- **THEN** they import `github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup`
- **AND** this module has no `pkg/iplookup` package

### Requirement: Contains is same-family only
`Contains` SHALL answer longest-prefix membership on the tree for that IP's family only. IPv4 (including IPv4-mapped IPv6) and IPv6 prefixes SHALL live on separate trees. A stored prefix MUST NOT match an address of the other family. Callers SHALL pass the hop `net.IP` Plugin already chose (`IPHeaders` / `ipHeaderStrategy`); they MUST NOT re-parse the request to rebuild that address.

#### Scenario: IPv4 prefix does not match IPv6
- **WHEN** the helper stores `0.0.0.0/0` and not an IPv6 prefix
- **AND** `Contains` is called with an IPv6 address
- **THEN** found is false

#### Scenario: IPv6 prefix does not match IPv4
- **WHEN** the helper stores `::/0` and not an IPv4 prefix
- **AND** `Contains` is called with an IPv4 address
- **THEN** found is false

#### Scenario: Same-family longest prefix matches
- **WHEN** the helper stores `8.8.8.0/24`
- **AND** `Contains` is called with `8.8.8.8`
- **THEN** found is true
- **AND** prefix length is 24

### Requirement: Product owns directory CIDR lists
Plugin creation SHALL load `allowedIPBlocks` / `blockedIPBlocks` and, when set, every `.txt` file under `allowedIPBlocksDir` / `blockedIPBlocksDir` (one CIDR per line, `#` comments) into the utilities helper. A missing directory SHALL NOT fail creation. The utilities package SHALL NOT be required to publish a directory monitor.

#### Scenario: Static block loads
- **WHEN** `blockedIPBlocks` contains `8.8.8.8/32`
- **AND** the request IP is `8.8.8.8`
- **THEN** the request is blocked by the CIDR list

#### Scenario: Missing directory is not fatal
- **WHEN** `allowedIPBlocksDir` names a path that does not exist
- **AND** static `allowedIPBlocks` is empty
- **THEN** plugin creation succeeds
- **AND** the allowed list is empty

#### Scenario: Directory txt lines load
- **WHEN** `blockedIPBlocksDir` contains a `.txt` file with `1.1.1.0/24`
- **AND** the request IP is `1.1.1.1`
- **THEN** the request is blocked by the CIDR list
