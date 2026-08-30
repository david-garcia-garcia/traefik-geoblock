## Why

Operators who map `requestHeaderEnrich` `country` onto a second header (for example `X-Geo-Country` while `countryHeader` is `X-IPCountry`) fail plugin creation. Two headers with the same country value is intended; block still reads `countryHeader` only.

## What Changes

- Plugin creation succeeds when `requestHeaderEnrich` maps `country` onto headers other than `countryHeader`.
- Lookup writes `countryHeader` and every enrich `country` header.
- Block still reads `countryHeader` only.
- README and the request-mode usage packet stop requiring a single country enrich name.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_plugin_request-mode`: An extra `requestHeaderEnrich` `country` mapping SHALL be written. Plugin creation MUST NOT fail because more than one header maps to `country`.

## Impact

- `pkg/geoblock/config.go` — remove `checkCountryHeaderBridge`.
- `pkg/geoblock/plugin_mode_test.go` — allow extra country enrich; assert both headers written.
- `README.md` example comments.
- `knowledge/devdocs/core_geoblock_plugin_request-mode.md` Language and How to use.
