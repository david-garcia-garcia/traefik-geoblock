---
url: https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/server_entrypoint_tcp.go
title: pkg/server/server_entrypoint_tcp.go
fetched: 2026-08-28
authority: source
ref: github.com/traefik/traefik@83c3499fc31c96e9f80ea0bba4d975f608c7061d:pkg/server/server_entrypoint_tcp.go
---

`TCPEntryPoints.Switch` calls `SwitchRouter` per entrypoint.

`SwitchRouter` replaces HTTP/HTTPS handlers via `Switcher.UpdateHandler` and switches the TCP router. Old handlers are no longer the live mux. No plugin `Close` is invoked here.
