---
url: https://github.com/oschwald/maxminddb-golang/blob/a295f5b07f9e2e131ede7db07dbd592a225028c0/mmap_windows.go
title: maxminddb-golang mmap_windows.go
fetched: 2026-08-27
authority: source
ref: github.com/oschwald/maxminddb-golang@a295f5b07f9e2e131ede7db07dbd592a225028c0:mmap_windows.go
---

Windows mmap implementation imports `unsafe`. Maps the file, then:

```
addr := *(*unsafe.Pointer)(unsafe.Pointer(&addrUintptr))
return unsafe.Slice((*byte)(addr), length), nil
```

`munmap` uses `unsafe.SliceData` / `unsafe.Pointer` to recover the address. Relevant later if Traefik Yaegi cannot load `unsafe` or mmap.
