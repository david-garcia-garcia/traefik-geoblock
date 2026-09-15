---
url: https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/5bfd3224090f7d5dd88577ce4d6d8aac84a9afe3/iplookup/tree.go
title: iplookup tree.go
fetched: 2026-09-15
authority: source
ref: github.com/david-garcia-garcia/traefik-middleware-utilities@5bfd3224090f7d5dd88577ce4d6d8aac84a9afe3:iplookup/tree.go
---

`ipRadixTree` is one family's prefixes. It does not classify IPv4 vs IPv6.

`insert` walks prefix bits, creates missing children, stores endpoint + metadata. `added` is false when the prefix was already present (metadata replaced).

`contains` records each endpoint so the last one is the longest prefix. Same walk as insert.

`remove` clears the endpoint and prunes empty leaves toward the root.

`familyWalk` maps IPv4 (`To4() != nil`) to 16-byte form, bitStart 96, maxPrefixLen 32. IPv6 uses the address as-is, bitStart 0, maxPrefixLen 128.

`bitAt` is most-significant bit first in each byte.
