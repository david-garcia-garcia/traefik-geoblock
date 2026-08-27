---
url: https://github.com/traefik/plugindemo/blob/44419f66fe21c51f4c94fd46f8e02b98e4fb3168/readme.md
title: Developing a Traefik plugin
fetched: 2026-08-26
authority: official
ref: github.com/traefik/plugindemo@44419f66fe21c51f4c94fd46f8e02b98e4fb3168:readme.md
---

A Traefik middleware plugin is a Go package that provides an `http.Handler`. Plugins run in Yaegi, not as pre-compiled binaries.

A plugin package must export:

- `type Config struct { ... }`
- `func CreateConfig() *Config`
- `func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error)`

The example puts those symbols in one root package (`package example`).

`.traefik.yml` `import` (required): the import path of the plugin. Example: `github.com/username/my-plugin`. Demo manifest uses `import: github.com/traefik/plugindemo`.

Plugin dependencies must be vendored and committed. Go modules are not supported (for those dependencies).

Local mode is a GOPATH workspace. Plugins live under `./plugins-local/src/<module path>/` relative to Traefik’s working directory. Example:

```
./plugins-local/src/github.com/traefik/plugindemo/
```

Static config sets `experimental.localPlugins.<alias>.moduleName` to that Go package path.

The demo tree shown in the README is a single-package plugin (demo.go at the module root). The README does not forbid additional packages under that module path.
