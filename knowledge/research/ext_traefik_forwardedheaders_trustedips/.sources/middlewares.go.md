---
url: https://github.com/traefik/traefik/blob/faa1eb590646aed94e561e24a59be0c47353ae95/pkg/config/dynamic/middlewares.go
title: pkg/config/dynamic/middlewares.go (IPStrategy)
fetched: 2026-11-03
authority: source
ref: github.com/traefik/traefik@faa1eb590646aed94e561e24a59be0c47353ae95:pkg/config/dynamic/middlewares.go
---

`IPStrategy`: Depth (minimum 0) takes XFF from the right. ExcludedIPs: scan XFF and select the first IP not in the list. Comment: "Depth override the ExcludedIPs."

`(*IPStrategy) Get()`: nil receiver → `RemoteAddrStrategy`. `Depth > 0` → `DepthStrategy` (excludedIPs unused). Else if `ExcludedIPs` non-empty → `PoolStrategy` with those IPs as the checker pool. Else `RemoteAddrStrategy` (optional IPv6Subnet).
