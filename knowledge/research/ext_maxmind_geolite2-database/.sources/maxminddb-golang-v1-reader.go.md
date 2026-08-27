---
url: https://github.com/oschwald/maxminddb-golang/blob/v1.13.1/reader.go
title: maxminddb-golang v1.13.1 FromBytes
fetched: 2026-08-27
authority: source
ref: vendor/github.com/oschwald/maxminddb-golang/reader.go (module v1.13.1)
---

`FromBytes(buffer []byte) (*Reader, error)` takes a byte slice of a MaxMind DB file and returns a Reader. It does not memory-map; it decodes metadata from the buffer.

`Lookup` unmarshals into a struct using `maxminddb` tags. This tree’s `go.mod` pins `github.com/oschwald/maxminddb-golang v1.13.1`.
