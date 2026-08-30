# Request mode

## Language

**Mode**:
The Traefik Config field that selects lookup, block, both, or pass-through. Values are `disabled`, `enrich`, `block`, and `enrichandblock`. Empty is `enrichandblock`.
_Avoid_: `enabled`

**Country header**:
The request header name lookup writes and block reads. Empty config uses `X-IPCountry`.
_Avoid_: using the lookup `Record` country for allow/block

## Overview

One `ServeHTTP` runs two stages. Lookup writes `countryHeader` and `requestHeaderEnrich`. Block applies CIDR and private rules per IP, then country allow/block from `countryHeader`. `block` and `disabled` do not open catalog sources.

## How to use

- Set `Config.Mode`. Unknown values fail `Prepare`. Empty is `enrichandblock`. Set `disabled` for pass through with no database.
- Omit `countryHeader` to use `X-IPCountry`. Extra `requestHeaderEnrich` `country` mappings are written too. Block still reads `countryHeader` only.
- Call `openCatalogSources` / `bindDatabase` only when `ModeLooksUp` (`enrich`, `enrichandblock`).
- Write country from lookup onto `countryHeader`, then read that header in the block stage. Do not pass `Record.Country` into country maps.
- Do not call `writeDefaultEnrichHeaders` in `block` (it would overwrite the inbound country).
- After CIDR, a `countryHeader` value of `PRIVATE` follows `allowPrivate`. Private or loopback IPs still apply `allowPrivate` first.

## Pattern snippet

```go
if ModeLooksUp(p.mode) {
	lookupFailed = p.enrich(req, remoteIPs, ipChain)
}
if ModeBlocks(p.mode) && skipBlock == PhaseNone {
	if lookupFailed && p.banIfError {
		return
	}
	if p.blockFromHeader(rw, req, remoteIPs, ipChain) {
		return
	}
}
```

## Key files

- `pkg/geoblock/config.go` — `Mode`, `Prepare`, `foldCountryHeader`
- `pkg/geoblock/plugin.go` — `NewCore`, `enrich`, `blockFromHeader`, `decide`
- `pkg/geoblock/plugin_mode_test.go` — mode, header, and PRIVATE cases

## Gotchas

- Chain enrich before block. Missing `countryHeader` uses `banIfError`.
- Country rules use the one `countryHeader` value (first public written). `CheckAll` still applies CIDR and private per selected IP.
- `foldCountryHeader` copies `countryHeader` onto `requestHeaderEnrich` as `country` when that header name is unset.
