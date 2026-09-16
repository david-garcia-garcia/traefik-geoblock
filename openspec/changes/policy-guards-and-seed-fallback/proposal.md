## Why

Dest `decide` treats CIDR prefix length 0 as “no allow match”, so `0.0.0.0/0` beats a `/32` block (README says the more specific prefix wins). A request with no hop from `GetRemoteIPs` reaches the backend as `pass:none` even when `banIfError` is true. An unreadable dated catalog file fails plugin creation even when a valid seed `path` exists. `countryHeader` mapped to a non-country enrich key loads silently. Exported `Lookup` on `mode=block` panics on a nil catalog.

## What Changes

- `decide` compares allow and block prefix lengths including 0; the longer prefix wins.
- Empty `GetRemoteIPs` is a lookup-class error: warn, then `banIfError`. Do not invent a hop.
- When the dated Latest file cannot be opened, lifecycle falls back to catalog `path` then `defaultFile`, warns, and starts. Keep-current may retry Latest; a failed hot-swap stays on the seed.
- `Prepare` warns when `countryHeader` is also a `requestHeaderEnrich` mapping whose key is not `country`. Enrich still wins.
- `Lookup` / `CheckAllowed` on a block-mode plugin return an error that the catalog is not bound. No panic. `ServeHTTP` still reads `countryHeader` only.
- Empty `bypassHeaders` stays as dest (`579856d`). Delete leftover `zzz_proof_*` files; add ordinary tests. No `zzz_proof_*` names.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_plugin_request-mode`: more-specific CIDR wins; empty hop list uses `banIfError`; warn when `countryHeader` is filled with a non-country enrich key; block-mode `Lookup` errors instead of panicking.
- `core_geoblock_database_url-download`: unreadable dated Latest falls back to seed at create.

## Impact

- `pkg/geoblock/plugin.go` `decide`, `blockFromHeader`, `Lookup`
- `pkg/geoblock/config.go` `Prepare` / `foldCountryHeader`
- `pkg/dbwrappers/lifecycle.go` `initialize` / `publish`
- `pkg/geoblock` and `pkg/dbwrappers` tests; delete `zzz_proof_*`
- README / request-mode and source usage if operator-facing warn/fallback is documented
- **Operators:** a `/0` allow no longer exempts a more-specific block. A corrupt dated file no longer takes the middleware down. A Traefik config that maps `countryHeader` to `city` still loads, with a warning.
