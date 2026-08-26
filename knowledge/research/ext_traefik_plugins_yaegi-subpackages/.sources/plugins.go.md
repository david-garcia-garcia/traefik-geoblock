---
url: https://github.com/traefik/traefik/blob/14bc52dd1f1d1c08cedd1da531a527fc04d79c19/pkg/plugins/plugins.go
title: pkg/plugins/plugins.go
fetched: 2026-08-26
authority: source
ref: github.com/traefik/traefik@14bc52dd1f1d1c08cedd1da531a527fc04d79c19:pkg/plugins/plugins.go
---

`localGoPath = "./plugins-local/"`.

`checkLocalPluginManifest` reads the manifest via `ReadManifest(localGoPath, descriptor.ModuleName)`.

For Yaegi plugins:

- `import` must be non-empty.
- `import` must be a prefix of the configured `moduleName` (`strings.HasPrefix(m.Import, descriptor.ModuleName)`). Error: `the import %q must be related to the module name %q`.

So `import` may be the module root or a longer path under that module. It cannot point outside the module.
