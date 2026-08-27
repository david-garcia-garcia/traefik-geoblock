---
url: https://github.com/oschwald/geoip2-golang/blob/v1.13.0/reader.go
title: geoip2-golang v1.13.0 reader.go
fetched: 2026-08-27
authority: source
ref: github.com/oschwald/geoip2-golang@v1.13.0:reader.go
---

Package comment: easy API for MaxMind GeoIP2 and GeoLite2; structs match the internal database structure. Built on `github.com/oschwald/maxminddb-golang`.

`Country` nested struct: `` `maxminddb:"country"` `` with `` IsoCode `maxminddb:"iso_code"` ``, `names`, `geoname_id`, `is_in_european_union`. Same nest on `City`. `ASN` is flat: `autonomous_system_number`, `autonomous_system_organization`.

`Open` uses `maxminddb.Open` (memory map). `FromBytes` uses `maxminddb.FromBytes` (no mmap). `Country(ip)` / `City(ip)` / `ASN(ip)` call `Lookup` into those structs.

Metadata `DatabaseType` values include `GeoLite2-Country`, `GeoLite2-City`, `GeoLite2-ASN`, `GeoIP2-Country`, `GeoIP2-City`.
