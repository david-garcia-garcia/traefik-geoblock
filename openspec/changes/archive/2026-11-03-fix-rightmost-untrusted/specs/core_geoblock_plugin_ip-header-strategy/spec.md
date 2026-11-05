## Purpose

Selects one client hop for enrich and block from `ipHeaders` by walking the collected chain from the right and skipping `trustedProxies`, so a client cannot pick the country by forging left-side hops.

## ADDED Requirements

### Requirement: RightmostUntrusted consumes the client-address owner's chain
When `ipHeaderStrategy` is `RightmostUntrusted`, the plugin SHALL take the hop chain from this plugin's client-address owner (`collectHops` / `rightmostChain` in `pkg/geoblock/ip.go`). That chain SHALL keep hop order and repeated addresses. The plugin MUST NOT rebuild the chain from `GetRemoteIPs` (first-occurrence dedup). The plugin MUST NOT copy Traefik `ipStrategy.excludedIPs`. An empty chain SHALL use the existing empty-hop path (`banIfError`). The plugin MUST NOT invent a hop.

The plugin SHALL walk that chain from the right and evaluate exactly the first hop that is not in `trustedProxies`.

#### Scenario: Repeated client IP does not hide a later untrusted hop
- **WHEN** `ipHeaderStrategy` is `RightmostUntrusted`, `trustedProxies` is `10.10.60.0/24`, and `blockedCountries` includes `US`
- **AND** `X-Forwarded-For` is `8.8.8.8, 1.1.1.1, 8.8.8.8`
- **THEN** the request is blocked
- **AND** `countryHeader` is `US`

#### Scenario: Same IP on header and RemoteAddr is still one evaluated hop
- **WHEN** `ipHeaders` is `x-forwarded-for, remoteAddress`, `ipHeaderStrategy` is `RightmostUntrusted`, and `trustedProxies` is empty
- **AND** `X-Forwarded-For` is `1.1.1.1` and `RemoteAddr` is `1.1.1.1:5000`
- **THEN** the evaluated hop is `1.1.1.1`

### Requirement: RightmostUntrusted reads every field line of the selected header
On the `RightmostUntrusted` path, a forwarding header sent as several HTTP field lines SHALL be one list: every line in order, each split on `,`.

#### Scenario: Second X-Forwarded-For line is the rightmost hop
- **WHEN** `ipHeaderStrategy` is `RightmostUntrusted`, `blockedCountries` includes `US`, and `trustedProxies` is empty
- **AND** the request adds `X-Forwarded-For: 1.1.1.1` then `X-Forwarded-For: 8.8.8.8`
- **THEN** the request is blocked
- **AND** `countryHeader` is `US`

#### Scenario: Mixed comma lists across two field lines
- **WHEN** `ipHeaderStrategy` is `RightmostUntrusted` and `trustedProxies` is `10.10.60.0/24`
- **AND** the request adds `X-Forwarded-For: 1.1.1.1, 2.2.2.2` then `X-Forwarded-For: 8.8.8.8, 10.10.60.5`
- **THEN** the evaluated hop is `8.8.8.8`

### Requirement: Forwarding ipHeaders are alternatives and remoteAddress closes the chain
When `ipHeaderStrategy` is `RightmostUntrusted`, every `ipHeaders` entry except `remoteAddress` SHALL be an alternative, tried in config order. The first header that yields at least one hop SHALL be the selected header. A header that is absent, empty, or holds no hop after splitting and trimming SHALL be skipped. Later forwarding headers MUST NOT be concatenated into that chain.

When `remoteAddress` is listed, it SHALL be the last `ipHeaders` entry and SHALL be appended as the last hop of the chain. Plugin creation SHALL fail if `remoteAddress` is listed and is not last. Plugin creation SHALL succeed if `remoteAddress` is omitted; creation SHALL warn once that headers are trusted without checking the direct peer.

When no configured forwarding header is present, the chain SHALL be `RemoteAddr` if `remoteAddress` is listed, otherwise empty.

When `remoteAddress` is listed and the direct peer is not in `trustedProxies`, that peer SHALL be the evaluated hop and every forwarding header SHALL be ignored.

#### Scenario: First present forwarding header wins
- **WHEN** `ipHeaders` is `cf-connecting-ip, x-forwarded-for, remoteAddress`, `ipHeaderStrategy` is `RightmostUntrusted`, and `RemoteAddr` is in `trustedProxies`
- **AND** the request has `CF-Connecting-IP: 1.1.1.1` and `X-Forwarded-For: 8.8.8.8`
- **THEN** the evaluated hop is `1.1.1.1`

#### Scenario: Present but empty header is skipped
- **WHEN** `ipHeaders` is `cf-connecting-ip, x-forwarded-for, remoteAddress`, `ipHeaderStrategy` is `RightmostUntrusted`, and `RemoteAddr` is in `trustedProxies`
- **AND** the request has `CF-Connecting-IP` with an empty value and `X-Forwarded-For: 8.8.8.8`
- **THEN** the evaluated hop is `8.8.8.8`

#### Scenario: Later header is used when the earlier one is absent
- **WHEN** `ipHeaders` is `cf-connecting-ip, x-forwarded-for, remoteAddress`, `ipHeaderStrategy` is `RightmostUntrusted`, and `RemoteAddr` is in `trustedProxies`
- **AND** the request has only `X-Forwarded-For: 8.8.8.8`
- **THEN** the evaluated hop is `8.8.8.8`

#### Scenario: Untrusted direct peer ignores forged headers
- **WHEN** `ipHeaders` is `cf-connecting-ip, x-forwarded-for, remoteAddress`, `ipHeaderStrategy` is `RightmostUntrusted`, and `RemoteAddr` `8.8.8.8:5000` is not in `trustedProxies`
- **AND** the request has `X-Forwarded-For: 1.1.1.1`
- **THEN** the evaluated hop is `8.8.8.8`

#### Scenario: Default ipHeaders stay valid
- **WHEN** `ipHeaders` is `x-forwarded-for, x-real-ip` and `ipHeaderStrategy` is `RightmostUntrusted`
- **THEN** plugin creation succeeds
- **AND** a request that carries both headers is evaluated from `X-Forwarded-For` only

#### Scenario: remoteAddress not last fails creation
- **WHEN** `ipHeaders` is `remoteAddress, x-forwarded-for` and `ipHeaderStrategy` is `RightmostUntrusted`
- **THEN** plugin creation fails

#### Scenario: Missing remoteAddress is a warning not a reject
- **WHEN** `ipHeaders` is `x-forwarded-for` and `ipHeaderStrategy` is `RightmostUntrusted`
- **THEN** plugin creation succeeds
- **AND** creation logs one warning that headers are trusted without checking the direct peer

#### Scenario: No forwarding header present uses RemoteAddr
- **WHEN** `ipHeaders` is `x-forwarded-for, remoteAddress` and `ipHeaderStrategy` is `RightmostUntrusted`
- **AND** the request has no `X-Forwarded-For` and `RemoteAddr` is `8.8.8.8:5000` not in `trustedProxies`
- **THEN** the evaluated hop is `8.8.8.8`

### Requirement: trustedProxies load and membership
`trustedProxies` set (non-empty after trimming) with a strategy other than `RightmostUntrusted` SHALL fail plugin creation. Empty `trustedProxies` with `RightmostUntrusted` SHALL succeed.

Plugin creation SHALL parse `trustedProxies` once onto the shared family-isolated CIDR helper through `iplookup.Helper.AddEntry`. A plain address SHALL be stored as its host prefix: `/32` for IPv4 and IPv4-mapped IPv6 (an IPv4-mapped literal is the IPv4 host), `/128` for IPv6. An IPv4-mapped CIDR SHALL load without error. An invalid entry SHALL fail creation and name that entry. There SHALL NOT be a second `trustedProxies` parser.

Membership SHALL use that helper's `Contains` on the hop `net.IP` the client-address owner already chose. A `Contains` error SHALL treat the hop as not trusted (the hop is evaluated, not skipped). An IPv4 prefix MUST NOT match an IPv6 hop.

When every hop in the chain is trusted, the leftmost hop (where the request started) SHALL be the evaluated hop. It MUST NOT use the empty-hop path.

An unparsable rightmost untrusted hop SHALL still be the evaluated hop. With `banIfError` true the request is blocked; with `banIfError` false, `defaultAllow` SHALL decide it. It MUST NOT pass without a decision.

#### Scenario: trustedProxies with CheckAll fails creation
- **WHEN** `trustedProxies` is `10.0.0.0/8` and `ipHeaderStrategy` is `CheckAll`
- **THEN** plugin creation fails

#### Scenario: IPv4 /0 does not trust an IPv6 hop
- **WHEN** `ipHeaderStrategy` is `RightmostUntrusted` and `trustedProxies` is `0.0.0.0/0`
- **AND** the rightmost hop is an IPv6 client address
- **THEN** that IPv6 hop is evaluated

#### Scenario: Bare IPv6 address matches only that address
- **WHEN** `ipHeaderStrategy` is `RightmostUntrusted` and `trustedProxies` is `2001:db8::1`
- **AND** the chain ends with `2001:db8::1` then `2001:db8::2`
- **THEN** `2001:db8::1` is skipped
- **AND** the evaluated hop is `2001:db8::2`

#### Scenario: Invalid trustedProxies entry fails creation
- **WHEN** `ipHeaderStrategy` is `RightmostUntrusted` and `trustedProxies` contains `not-an-ip`
- **THEN** plugin creation fails and the error names `not-an-ip`

#### Scenario: Prefix length 33 fails creation
- **WHEN** `ipHeaderStrategy` is `RightmostUntrusted` and `trustedProxies` contains `10.0.0.0/33`
- **THEN** plugin creation fails and the error names `10.0.0.0/33`

#### Scenario: Bare IPv4-mapped address trusts the IPv4 host
- **WHEN** `ipHeaderStrategy` is `RightmostUntrusted` and `trustedProxies` is `::ffff:10.10.60.5`
- **AND** the chain is `8.8.8.8, 10.10.60.5`
- **THEN** the evaluated hop is `8.8.8.8`
- **AND** a chain ending in `::5` evaluates `::5`

#### Scenario: Health check from a trusted proxy follows allowPrivate
- **WHEN** `ipHeaders` is `x-forwarded-for, remoteAddress`, `ipHeaderStrategy` is `RightmostUntrusted`, `trustedProxies` is `10.10.60.0/24`, `allowPrivate` is true, and `banIfError` is true
- **AND** the request has no `X-Forwarded-For` and `RemoteAddr` is `10.10.60.5:443`
- **THEN** the evaluated hop is `10.10.60.5`
- **AND** the request passes with reason `allow_private`
- **AND** no empty-hop warning is logged

#### Scenario: Unparsable hop with banIfError false uses defaultAllow
- **WHEN** `ipHeaderStrategy` is `RightmostUntrusted`, `banIfError` is false, and `defaultAllow` is false
- **AND** the evaluated hop is `garbage`
- **THEN** the request is blocked with reason `default_allow`

#### Scenario: Empty chain uses banIfError
- **WHEN** `ipHeaders` is `x-forwarded-for`, `ipHeaderStrategy` is `RightmostUntrusted`, and `banIfError` is true
- **AND** the request has no `X-Forwarded-For`
- **THEN** the request is blocked
