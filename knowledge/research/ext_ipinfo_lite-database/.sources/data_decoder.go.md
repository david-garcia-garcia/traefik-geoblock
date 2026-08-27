---
url: https://github.com/oschwald/maxminddb-golang/blob/a295f5b07f9e2e131ede7db07dbd592a225028c0/internal/decoder/data_decoder.go
title: maxminddb-golang data_decoder.go decodeKey
fetched: 2026-08-27
authority: source
ref: github.com/oschwald/maxminddb-golang@a295f5b07f9e2e131ede7db07dbd592a225028c0:internal/decoder/data_decoder.go
---

`decodeKey` comment: map keys are decoded into `[]byte` to avoid a copy when decoding a struct (Go issue 3512). “Previously, we achieved this by using unsafe.” Current decode path does not use `unsafe` for that.
