---
url: https://github.com/oschwald/maxminddb-golang/blob/a295f5b07f9e2e131ede7db07dbd592a225028c0/go.mod
title: maxminddb-golang go.mod
fetched: 2026-08-27
authority: source
ref: github.com/oschwald/maxminddb-golang@a295f5b07f9e2e131ede7db07dbd592a225028c0:go.mod
---

```
module github.com/oschwald/maxminddb-golang/v2
go 1.25.0
```

Requires `golang.org/x/sys v0.47.0` (used by Unix mmap). Decoder comment at `internal/decoder/data_decoder.go` says map-key decoding previously used `unsafe` and no longer does.
