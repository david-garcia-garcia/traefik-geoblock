## 1. Bump and vendor utilities v1.0.2

- [x] 1.1 Require `github.com/david-garcia-garcia/traefik-middleware-utilities` v1.0.2 in `go.mod`
- [x] 1.2 Run `go mod vendor` and re-apply `scripts/apply-oschwald-yaegi-patch.ps1`

## 2. Switch geoblock onto the vendored helper

- [x] 2.1 In `pkg/geoblock`, load static CIDRs then directory `.txt` files onto `iplookup.New()` (`AddCIDR(cidr, "")`; missing dir is debug, not fatal)
- [x] 2.2 Store `*iplookup.Helper` on Plugin; `isAllowedIPBlocks` / `isBlockedIPBlocks` call `Contains` (discard metadata)
- [x] 2.3 Update `plugin_observe_test.go` empty-list constructions to `iplookup.New()`

## 3. Tests and delete local package

- [x] 3.1 Port missing-dir, static+directory, and invalid-line cases into `pkg/geoblock` tests
- [x] 3.2 Delete `pkg/iplookup/`

## 4. Usage docs

- [x] 4.1 Update `knowledge/devdocs/core_geoblock_plugin_packages.md` to drop local `iplookup` and point at vendored utilities `iplookup`
