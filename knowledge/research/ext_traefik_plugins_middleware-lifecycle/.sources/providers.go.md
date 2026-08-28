---
url: https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/plugins/providers.go
title: pkg/plugins/providers.go
fetched: 2026-08-28
authority: source
ref: github.com/traefik/traefik@83c3499fc31c96e9f80ea0bba4d975f608c7061d:pkg/plugins/providers.go
---

Provider plugin interface `PP`: `Init()`, `Provide(...)`, `Stop() error`.

Traefik calls `Stop()` when the provider plugin’s context is done. This is the provider plugin type, not HTTP middleware.
