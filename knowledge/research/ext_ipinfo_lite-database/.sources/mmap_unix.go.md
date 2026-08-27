---
url: https://github.com/oschwald/maxminddb-golang/blob/a295f5b07f9e2e131ede7db07dbd592a225028c0/mmap_unix.go
title: maxminddb-golang mmap_unix.go
fetched: 2026-08-27
authority: source
ref: github.com/oschwald/maxminddb-golang@a295f5b07f9e2e131ede7db07dbd592a225028c0:mmap_unix.go
---

Build tag: `!windows && !appengine && !plan9 && !js && !wasip1 && !wasi`.

`mmap` calls `unix.Mmap(fd, 0, length, PROT_READ, MAP_SHARED)`. `munmap` calls `unix.Munmap`. ENODEV is treated as `errors.ErrUnsupported` so `Open` can fall back.

This file does not import `unsafe`.
