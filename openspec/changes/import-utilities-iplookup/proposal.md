## Why

DestBranch stores CIDR allow/block lists in a first-party radix on one shared tree, so a prefix can match the other family. Utilities v1.0.2 already publishes a family-isolated `iplookup` helper. PR 86 is closed; isolation ships by consuming that package, not by rewriting the local helper.

## What Changes

- Bump `github.com/david-garcia-garcia/traefik-middleware-utilities` from v1.0.1 to v1.0.2 and vendor it (Yaegi). Re-apply `scripts/apply-oschwald-yaegi-patch.ps1` after `go mod vendor`.
- Switch product call sites from `github.com/david-garcia-garcia/traefik-geoblock/pkg/iplookup` to `github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup`.
- **BREAKING** (internal): delete first-party `pkg/iplookup`. Static + directory `.txt` load moves into `pkg/geoblock`. Plugin fields hold `*iplookup.Helper`.
- Preserve `allowedIPBlocks` / `blockedIPBlocks` and directory-list behavior. `decide` `/0` vs longest-prefix stays. Hop IP stays on Plugin `IPHeaders` / `ipHeaderStrategy`.
- Update `knowledge/devdocs/core_geoblock_plugin_packages.md` so it no longer lists a local `iplookup` helper.

## Capabilities

### New Capabilities

- `std_go_iplookup_family-isolated`: this module imports vendored utilities `iplookup` at the `go.mod` pin, keeps IPv4 and IPv6 on separate trees, and SHALL NOT keep a first-party `pkg/iplookup` package.

### Modified Capabilities

None.

## Impact

- `go.mod`, `go.sum`, `vendor/github.com/david-garcia-garcia/traefik-middleware-utilities/`
- Delete `pkg/iplookup/`
- `pkg/geoblock/plugin.go`, `pkg/geoblock/plugin_observe_test.go`, new geoblock directory-load tests
- `knowledge/devdocs/core_geoblock_plugin_packages.md`
