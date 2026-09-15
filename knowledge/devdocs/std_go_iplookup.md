# Family-isolated CIDR helper

## Language

**Helper**:
A CIDR store with one tree per address family. `Contains` answers longest-prefix membership on the same family only.
_Avoid_: first-party `pkg/iplookup`; one shared radix for IPv4 and IPv6

## Overview

The helper is `github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup`, pinned in `go.mod` and loaded from `vendor/`. Do not copy it into `pkg/iplookup`. `go mod vendor` omits `*_test.go`; library tests stay upstream. This plugin loads `allowedIPBlocks` / `blockedIPBlocks` and directory `.txt` lists onto it in `pkg/geoblock`.

## How to use

- Import `github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup`. Construct with `New()`. Store prefixes with `AddCIDR(cidr, "")`. Look up with `Contains` and discard metadata. Pass the hop `net.IP` Plugin already chose.
- Load static Config lists then directory `.txt` files through `loadIPBlockHelper`. A missing directory is debug, not fatal. After a version bump, `go mod vendor` and re-apply `scripts/apply-oschwald-yaegi-patch.ps1`.
- Do not import unused utilities packages. Do not reopen PR 86 or rewrite a local radix.

## Pattern snippet

```go
helper := iplookup.New()
if err := helper.AddCIDR(cidr, ""); err != nil {
	return err
}
found, prefixLen, _, err := helper.Contains(ipAddr)
```

## Key files

- `vendor/github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup`
- `pkg/geoblock/ip_blocks.go` — `loadIPBlockHelper`
- `pkg/geoblock/plugin.go` — `allowedIPBlocks` / `blockedIPBlocks`

## Gotchas

- `Contains` is same-family only. An IPv4 `/0` does not match IPv6.
- `decide` still treats prefix length `0` as the `/0` sentinel. Do not change that here.
- Yaegi loads this package from `vendor/`. Do not fork it into `pkg/`.
