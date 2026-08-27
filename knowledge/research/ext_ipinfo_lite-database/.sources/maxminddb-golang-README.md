---
url: https://github.com/oschwald/maxminddb-golang/blob/a295f5b07f9e2e131ede7db07dbd592a225028c0/README.md
title: MaxMind DB Reader for Go README
fetched: 2026-08-27
authority: source
ref: github.com/oschwald/maxminddb-golang@a295f5b07f9e2e131ede7db07dbd592a225028c0:README.md
---

Module `github.com/oschwald/maxminddb-golang/v2`. Not an official MaxMind API. Install: `go get github.com/oschwald/maxminddb-golang/v2`.

v2 API: `maxminddb.Open`, `netip.Addr`, `db.Lookup(ip).Decode(&record)`. Path decode example uses GeoIP2 keys: `DecodePath(&countryCode, "country", "iso_code")`.

Supported third-party databases include “IPinfo databases” (MaxMind DB format files). Format-agnostic for any valid `.mmdb`.
