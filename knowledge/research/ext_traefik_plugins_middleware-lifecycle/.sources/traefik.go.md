---
url: https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/cmd/traefik/traefik.go
title: cmd/traefik/traefik.go
fetched: 2026-08-28
authority: source
ref: github.com/traefik/traefik@83c3499fc31c96e9f80ea0bba4d975f608c7061d:cmd/traefik/traefik.go
---

Plugin builder is created once during process startup (`createPluginBuilder`) and passed into `NewRouterFactory`.

`watcher.AddListener(switchRouter(...))`. `switchRouter` on every applied dynamic config: `CreateRouters(rtConf)` then `serverEntryPointsTCP.Switch` / UDP `Switch`.
