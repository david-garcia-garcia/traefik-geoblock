---
url: https://github.com/traefik/traefik/blob/14bc52dd1f1d1c08cedd1da531a527fc04d79c19/pkg/plugins/types.go
title: pkg/plugins/types.go
fetched: 2026-08-26
authority: source
ref: github.com/traefik/traefik@14bc52dd1f1d1c08cedd1da531a527fc04d79c19:pkg/plugins/types.go
---

`Manifest` YAML fields used by the Yaegi loader:

- `import` (`Import`)
- `basePkg` (`BasePkg`)
- `runtime` (`Runtime`) — empty or `yaegi` means Yaegi (`IsYaegiPlugin`)
- `useUnsafe` (`UseUnsafe`)

`basePkg` is optional. `import` is the Go import path Traefik passes to Yaegi.
