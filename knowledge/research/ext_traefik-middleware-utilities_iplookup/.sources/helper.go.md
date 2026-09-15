---
url: https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/5bfd3224090f7d5dd88577ce4d6d8aac84a9afe3/iplookup/helper.go
title: iplookup helper.go
fetched: 2026-09-15
authority: source
ref: github.com/david-garcia-garcia/traefik-middleware-utilities@5bfd3224090f7d5dd88577ce4d6d8aac84a9afe3:iplookup/helper.go
---

Package is a Yaegi-safe CIDR store. IPv4 and IPv6 prefixes live on separate trees so a prefix cannot match the other family.

`Helper` stores CIDR prefixes and returns the winning longest-prefix match. Callers pass `net.IP` they already selected; Helper does not read HTTP.

`New()` returns an empty Helper with two trees.

`treeFor` returns the IPv4 tree when `ip.To4() != nil` (including IPv4-mapped IPv6).

`AddCIDR(cidr, metadata string)` parses CIDR, locks, inserts on that family's tree. Same canonical prefix replaces metadata and does not bump Count.

`RemoveCIDR` drops one stored prefix. Missing prefix returns false, nil error.

`Contains` returns longest stored prefix of the same family. Nil IP returns error `iplookup: IP address is nil`. Signature: `(found bool, prefixLen int, metadata string, err error)`.

`Reset` replaces both trees and zeros count.

`Count` is the number of stored prefixes, not the number of AddCIDR calls.
