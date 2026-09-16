# Request mode

## Language

**Mode**:
The Traefik Config field that selects lookup, block, both, or pass-through. Values are `disabled`, `enrich`, `block`, and `enrichandblock`. Empty is `enrichandblock`.
_Avoid_: `enabled`

**Country header**:
The request header name lookup writes and block reads. Empty config uses `X-IPCountry`.
_Avoid_: using the lookup `Record` country for allow/block

**BypassHeaders**:
A Config map of request header names to expected values. A present header whose first value equals the configured value skips the block stage.
_Avoid_: treating an omitted header as a match; using `Get == ""` as absent

## Overview

One `ServeHTTP` runs two stages. Lookup writes `countryHeader` and `requestHeaderEnrich`. Block applies CIDR and private rules per IP, then country allow/block from `countryHeader`. `block` and `disabled` do not open catalog sources. A `blockSkipReason` match (ignore verb, path regex, or BypassHeaders) skips the block stage; enrich still runs.

## How to use

- Set `Config.Mode`. Unknown values fail `Prepare`. Empty is `enrichandblock`. Set `disabled` for pass through with no database.
- Reject any `BypassHeaders` value whose `TrimSpace` is empty. An empty map is valid. `disabled` still returns before that check.
- Skip the block stage only when `blockSkipReason` is not `PhaseNone`. For a BypassHeaders pair, require `req.Header.Values(header)` non-empty, then compare `Get` to the configured value. Do not skip empty `expectedValue` at request time.
- Omit `countryHeader` to use `X-IPCountry`. Extra `requestHeaderEnrich` `country` mappings are written too. Block still reads `countryHeader` only.
- Call `openCatalogSources` / `bindDatabase` only when `ModeLooksUp` (`enrich`, `enrichandblock`).
- Write country from lookup onto `countryHeader`, then read that header in the block stage. Do not pass `Record.Country` into country maps.
- Do not call `writeDefaultEnrichHeaders` in `block` (it would overwrite the inbound country).
- After CIDR, a `countryHeader` value of `PRIVATE` follows `allowPrivate`. Private or loopback IPs still apply `allowPrivate` first.
- When both CIDR lists match, the longer prefix wins. Length 0 (`0.0.0.0/0`) is a match, so a `/32` block beats a `/0` allow. Equal lengths keep allow-before-block.
- Empty `GetRemoteIPs` is a lookup-class error: warn, then `banIfError`. Do not invent a hop.
- After `foldCountryHeader`, if `countryHeader` maps to a non-country enrich key, warn and keep that mapping.
- `Lookup` / `CheckAllowed` on `mode=block` return that the catalog is not bound. Do not dereference a nil catalog.
- A public IP whose merged lookup returns an empty country is written as `XX`, never `PRIVATE`, so it reaches the country maps and `defaultAllow`. `writePublicLookupHeaders` writes `XX` without marking the country written, so a later hop that does resolve still wins. `Combined` fills only empty fields. A `bin` `country_short` `-` is empty, so an enabled `bin` row does not pre-empt a later source or `XX`.

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

```go
for header, expectedValue := range p.bypassHeaders {
	if len(req.Header.Values(header)) == 0 {
		continue
	}
	if actualValue := req.Header.Get(header); actualValue == expectedValue {
		return PhaseBypassHeader
	}
}
```

## Key files

- `pkg/geoblock/config.go` — `Mode`, `Prepare`, `foldCountryHeader`, `BypassHeaders`
- `pkg/geoblock/plugin.go` — `NewCore`, `enrich`, `blockFromHeader`, `decide`, `blockSkipReason`
- `pkg/geoblock/plugin_mode_test.go` — mode, header, and PRIVATE cases

## Gotchas

- Chain enrich before block. Missing `countryHeader` or an empty hop list uses `banIfError`.
- `Header.Get` is `""` when the header is absent, so an empty configured value would match every omitted header. Presence is `len(Values) > 0`. Prepare is the load-time fail; the presence check covers a leftover empty map entry (`NewCore` without `Prepare`). A leftover empty entry must not write `pass:bypass_header`.
- Country rules use the one `countryHeader` value (first public written). `CheckAll` still applies CIDR and private per selected IP.
- Every selected hop can deny: `blockFromHeader` returns on the first hop `decide` rejects. `passReason` names the first *allowing* phase for the `pass:{reason}` header and must not gate the deny.
- `decide` answers private and loopback hops from `allowPrivate` **before** the CIDR lists, so `allowedIPBlocks` cannot allow a private hop.
- `enrich` writes the lookup record to the enrich headers **before** it checks the returned error, so `recordForLookup` returns no country on its error paths. `writePublicLookupHeaders` maps an empty country to `XX` without marking it written.
- `foldCountryHeader` copies `countryHeader` onto `requestHeaderEnrich` as `country` when that header name is unset.
