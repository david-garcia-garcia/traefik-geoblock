# Family-isolated CIDR helper

Pinned source: [traefik-middleware-utilities](https://github.com/david-garcia-garcia/traefik-middleware-utilities) tag **v1.0.2**, commit `5bfd3224090f7d5dd88577ce4d6d8aac84a9afe3`. The published package is `github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup`. Implementation files: `iplookup/helper.go`, `iplookup/tree.go` (stdlib: `errors`, `fmt`, `net`, `sync`). Extracts: [`.sources/helper.go.md`](.sources/helper.go.md), [`.sources/tree.go.md`](.sources/tree.go.md), [`.sources/release-v1.0.2.md`](.sources/release-v1.0.2.md).

## Public surface

Source: [`iplookup/helper.go`](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/5bfd3224090f7d5dd88577ce4d6d8aac84a9afe3/iplookup/helper.go) at `5bfd322`.

- `Helper` — two trees (`ipv4Tree`, `ipv6Tree`), `count`, mutex.
- `New() *Helper` — empty helper.
- `AddCIDR(cidr, metadata string) error` — parse, store on the family tree for that network. A second store of the same canonical prefix replaces metadata and does not bump `Count`.
- `RemoveCIDR(cidr string) (removed bool, err error)` — missing prefix returns `false, nil`.
- `Contains(ipAddr net.IP) (found bool, prefixLen int, metadata string, err error)` — longest stored prefix of the **same family**. Nil IP is an error.
- `Reset()` — new empty trees, count 0.
- `Count() int` — stored prefixes, not AddCIDR call count.

There is no `IpLookupHelper`, `NewIpLookupHelper`, `IsContained`, or directory/file monitor in this package at v1.0.2.

## Family isolation

Source: [`iplookup/helper.go`](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/5bfd3224090f7d5dd88577ce4d6d8aac84a9afe3/iplookup/helper.go) `treeFor` and [`iplookup/tree.go`](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/5bfd3224090f7d5dd88577ce4d6d8aac84a9afe3/iplookup/tree.go) `familyWalk`.

IPv4 (including IPv4-mapped IPv6) and IPv6 prefixes live on separate trees. `treeFor` picks the tree from `ip.To4() != nil`. A prefix cannot match the other family. Each tree still walks IPv4 as 16-byte mapped form starting at bit 96.

## Release

Source: [v1.0.2](https://github.com/david-garcia-garcia/traefik-middleware-utilities/releases/tag/v1.0.2). Changelog: `5bfd322` feat(iplookup): add family-isolated CIDR helper with remove, labels, and reset (#94). Published 2026-09-15T07:50:24Z.
