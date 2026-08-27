---
url: https://github.com/oschwald/maxminddb-golang/blob/a295f5b07f9e2e131ede7db07dbd592a225028c0/reader.go
title: maxminddb-golang reader.go Open/OpenBytes
fetched: 2026-08-27
authority: source
ref: github.com/oschwald/maxminddb-golang@a295f5b07f9e2e131ede7db07dbd592a225028c0:reader.go
---

`Open(file)`: “The database file is opened using a memory map on supported platforms.” Fallback to load-into-memory when mmap is unsupported (comment: WebAssembly, Google App Engine) or the filesystem rejects mmap. Then `OpenBytes`.

`OpenBytes(buffer []byte)` builds a Reader from an in-memory slice (no mmap).

`openMmap` uses `SyscallConn` + platform `mmap`. Mapped readers set `hasMappedFile` and `munmap` on `Close` / cleanup.
