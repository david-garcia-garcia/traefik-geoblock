# Request mode

## Language

**Mode**:
The Traefik Config field that selects lookup, block, both, or pass-through. Values are `disabled`, `enrich`, `block`, and `enrichandblock`. Empty is `disabled`.
_Avoid_: `enabled`

**Country header**:
The required request header name when mode is not `disabled`. Lookup writes the ISO country or `PRIVATE` here. The block stage reads that same header for country allow/block.
_Avoid_: inventing `X-Geoblock-Country`; mapping a second `requestHeaderEnrich` header to `country`

## Overview

One `ServeHTTP` runs two stages. Lookup writes `countryHeader` and `requestHeaderEnrich`. Block applies CIDR and private rules per IP, then country allow/block from `countryHeader`. `block` and `disabled` do not open a DatabaseProvider.

## How to use

- Set `Config.Mode`. Unknown values fail `Prepare`. Empty is `disabled` (pass through, no database).
- Require `countryHeader` when mode is not `disabled`. A `requestHeaderEnrich` `country` mapping must use that same header name.
- Call `openDatabaseProvider` / `bindDatabase` only when `ModeLooksUp` (`enrich`, `enrichandblock`).
- Write country from lookup onto `countryHeader`, then read that header in the block stage. Do not pass `Record.Country` into country maps.
- Do not call `setPrivateGeoHeaders` in `block` (it would overwrite the inbound country).
- After CIDR, a `countryHeader` value of `PRIVATE` follows `allowPrivate`. Private or loopback IPs still apply `allowPrivate` first.

## Pattern snippet

```go
if ModeLooksUp(p.mode) {
	if p.lookupAndWriteHeaders(rw, req, remoteIPs, ipChain, passReason) {
		return
	}
}
if ModeBlocks(p.mode) && passReason == PhaseNone {
	if p.blockFromHeader(rw, req, remoteIPs, ipChain) {
		return
	}
}
```

## Key files

- `pkg/geoblock/config.go` — `Mode`, `Prepare`, `checkCountryHeaderBridge`
- `pkg/geoblock/plugin.go` — `NewCore`, `lookupAndWriteHeaders`, `blockFromHeader`, `decide`
- `pkg/geoblock/plugin_mode_test.go` — mode, header, and PRIVATE cases

## Gotchas

- Chain enrich before block. Missing `countryHeader` uses `banIfError`.
- Country rules use the one `countryHeader` value (first public written). `CheckAll` still applies CIDR and private per selected IP.
- `foldCountryHeader` copies `countryHeader` onto `requestHeaderEnrich` as `country` when that header name is unset.
