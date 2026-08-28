---
url: https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/configurationwatcher.go
title: pkg/server/configurationwatcher.go
fetched: 2026-08-28
authority: source
ref: github.com/traefik/traefik@83c3499fc31c96e9f80ea0bba4d975f608c7061d:pkg/server/configurationwatcher.go
---

`receiveConfigurations` skips a provider message when `reflect.DeepEqual` to that provider’s last configuration (`Skipping unchanged configuration`).

`applyConfigurations` skips when the full provider set is `DeepEqual` to `lastConfigurations`. Otherwise it merges and calls every listener, including `switchRouter`.

Skip is whole-tree equality, not “plugin middleware block unchanged.”
