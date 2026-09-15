## Why

`recordForLookup` returns `dbprovider.Record{Country: ip}` on both of its error paths, and `enrich` writes that record to the country enrich headers **before** it checks the returned error. The IP header value is caller-controlled, so `countryHeader` becomes whatever the client sent.

In `mode: enrich` with every setting at its default, `X-Forwarded-For: Norway` reaches the backend as `X-IPCountry: Norway`. The header is the plugin's product in that mode, and any client that can set an IP header sets it.

This contradicts `core_geoblock_plugin_request-mode`, which enumerates what lookup may write there: "the ISO country, `PRIVATE` for a private or loopback hop, or `XX` for a public hop the enabled sources returned no country for". A raw token is none of those. `decide`'s own parse-failure path already returns no country, and `pkg/geoblock/plugin_policy_test.go` already asserts that a provider lookup of a bad address yields an **empty** country — `recordForLookup` overrides that one layer up.

The value was unobservable when it was written: at the initial import there was no `countryHeader`, and the caller checked the error and discarded the record. PR #11 added the header write before the error check, and #66 made the block stage read the header back as the country.

## What Changes

- Both error returns in `recordForLookup` carry no country. `writePublicLookupHeaders` already maps an empty country to `XX` without marking the country written, so an unresolvable hop is `XX` like any other and a later hop that does resolve still wins.
- No config change, no new option. `banIfError` still fires: the error is returned unchanged, so `lookupFailed` and the in-loop ban are untouched.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_plugin_request-mode`: the countryHeader value set is stated as exhaustive — an IP header value the plugin cannot parse, or an address whose lookup errors, never reaches the header.

## Impact

- `pkg/geoblock/plugin.go` (`recordForLookup`)
- `pkg/geoblock/plugin_mode_test.go`
- `README.md`, `knowledge/devdocs/core_geoblock_plugin_request-mode.md`
- `geoblockban.html` `data-country="{{.Country}}"` via `serveBanHtml`, which interpolates without escaping: the token reached the ban page body and could leave the attribute. Same-sender only, so not cross-user. The escaping itself is out of scope — `mode: block` must pass an inbound `X-IPCountry` through unchanged.
- **Operators:** `countryHeader` changes from the raw token to `XX` for an unparseable hop. Anything downstream that matched on the token stops matching. With `defaultAllow: false` and `banIfError: false`, a chain led by a token that happened to name an allowed country stops passing — it was passing on a client-supplied value.
