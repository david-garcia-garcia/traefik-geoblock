# Yaegi plugin subpackages

A Traefik Yaegi plugin can import **Go subpackages of its own module**. Traefik only `Eval`-imports the manifest `import` path (usually the module root) and then looks up `New` and `CreateConfig` on that package. Helper code may live in subpackages such as `pkg/…` as long as those packages sit on the plugin GOPATH.

## Entry points stay on the imported package

Official catalog and demo: a Yaegi plugin is a Go package. That package must export `Config`, `CreateConfig`, and `New` ([Developing Traefik Plugins](https://plugins.traefik.io/create), [plugindemo readme](https://github.com/traefik/plugindemo/blob/44419f66fe21c51f4c94fd46f8e02b98e4fb3168/readme.md)). Manifest `import` is “the import path of your plugin.”

Traefik does not scan the repo for those symbols. At [traefik@14bc52dd:pkg/plugins/middlewareyaegi.go](https://github.com/traefik/traefik/blob/14bc52dd1f1d1c08cedd1da531a527fc04d79c19/pkg/plugins/middlewareyaegi.go) it:

1. Builds a Yaegi interpreter with `interp.Options{GoPath: goPath}`.
2. Runs `import "<manifest.Import>"`.
3. Evals `basePkg.New` and `basePkg.CreateConfig`.

If `basePkg` is empty, Traefik sets it to the last path segment of `import` with `-` replaced by `_` (so `crowdsec-bouncer-traefik-plugin` becomes `crowdsec_bouncer_traefik_plugin`).

Manifest fields `import` and `basePkg` are defined on [traefik@14bc52dd:pkg/plugins/types.go](https://github.com/traefik/traefik/blob/14bc52dd1f1d1c08cedd1da531a527fc04d79c19/pkg/plugins/types.go). Local Yaegi plugins must have a non-empty `import` that is a prefix of the configured module name ([traefik@14bc52dd:pkg/plugins/plugins.go](https://github.com/traefik/traefik/blob/14bc52dd1f1d1c08cedd1da531a527fc04d79c19/pkg/plugins/plugins.go)). That allows `import` to be the module root **or** a subpath of it. The usual catalog pattern is the module root.

Extracts: [.sources/create.md](.sources/create.md), [.sources/plugindemo-readme.md](.sources/plugindemo-readme.md), [.sources/middlewareyaegi.go.md](.sources/middlewareyaegi.go.md), [.sources/types.go.md](.sources/types.go.md), [.sources/plugins.go.md](.sources/plugins.go.md)

## Subpackages resolve through GOPATH, not Go modules

Yaegi `Options.GoPath` “sets GOPATH for the interpreter” ([yaegi@fcb76d1e:interp/interp.go](https://github.com/traefik/yaegi/blob/fcb76d1ece0c3edc2548c39aa5b170475d2261bb/interp/interp.go)). Yaegi’s README states Go modules are not supported yet and sources must live under `$GOPATH/src/…` ([yaegi README](https://github.com/traefik/yaegi/blob/fcb76d1ece0c3edc2548c39aa5b170475d2261bb/README.md)).

The official demo matches that layout. Local plugins go in a GOPATH workspace:

```
./plugins-local/src/<moduleName>/
```

([plugindemo readme](https://github.com/traefik/plugindemo/blob/44419f66fe21c51f4c94fd46f8e02b98e4fb3168/readme.md)). Traefik’s local GOPATH constant is `./plugins-local/` ([traefik@14bc52dd:pkg/plugins/plugins.go](https://github.com/traefik/traefik/blob/14bc52dd1f1d1c08cedd1da531a527fc04d79c19/pkg/plugins/plugins.go)).

Official “Go modules are not supported” / “dependencies must be vendored” is about **third-party** packages. It does not forbid the plugin module from containing its own subpackages. Those subpackages are extra directories under the same GOPATH `src/<module>/` tree and are imported as `<module>/pkg/…`.

Inference (from the files above): Traefik never `Eval`-imports helper subpackages itself. Yaegi loads them when the root package’s source imports them, using the interpreter GOPATH.

Extracts: [.sources/interp.go.md](.sources/interp.go.md), [.sources/yaegi-readme.md](.sources/yaegi-readme.md), [.sources/plugindemo-readme.md](.sources/plugindemo-readme.md), [.sources/plugins.go.md](.sources/plugins.go.md)

## Published plugin that uses `pkg/…`

[crowdsec-bouncer-traefik-plugin@ae7481ca](https://github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/tree/ae7481caa5a06e9fd31dcc5017f7dc0f9e2d00b3) is a catalog Yaegi plugin that follows this split.

- `.traefik.yml` `import` is the module root `github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin`.
- Root `bouncer.go` is `package crowdsec_bouncer_traefik_plugin` and exports `CreateConfig` and `New`.
- It imports helpers: `…/pkg/cache`, `…/pkg/captcha`, `…/pkg/configuration`, `…/pkg/ip`, `…/pkg/logger`.
- Those directories exist under `pkg/` (`package cache` in `pkg/cache/cache.go`).

This is third-party source evidence that Traefik Yaegi accepts the layout. It does not define Traefik’s loader.

Extracts: [.sources/traefik.yml.md](.sources/traefik.yml.md), [.sources/go.mod.md](.sources/go.mod.md), [.sources/bouncer.go.md](.sources/bouncer.go.md), [.sources/pkg.md](.sources/pkg.md), [.sources/cache.go.md](.sources/cache.go.md)

## Authority

| Claim | Owner | Rank |
| --- | --- | --- |
| Yaegi plugin is a Go package | https://plugins.traefik.io/create | official |
| Package must export `Config`, `CreateConfig`, `New` | plugindemo README | official |
| Manifest `import` is the plugin import path | plugindemo README | official |
| Third-party deps must be vendored; Go modules not supported | plugindemo README (same on create page) | official |
| Local plugin lives under `./plugins-local/src/<module>/` | plugindemo README | official |
| Traefik `Eval`s `import "<manifest.Import>"` then `basePkg.New` / `basePkg.CreateConfig` | traefik@14bc52dd:middlewareyaegi.go | source |
| Empty `basePkg` = last import segment, `-` → `_` | same | source |
| Interpreter `GoPath` is the plugin GOPATH | same | source |
| Manifest YAML fields `import`, `basePkg` | traefik@14bc52dd:types.go | source |
| Local `import` must be a prefix of `moduleName` | traefik@14bc52dd:plugins.go | source |
| Yaegi `Options.GoPath` sets interpreter GOPATH | yaegi@fcb76d1e:interp.go | source |
| Yaegi does not support Go modules; sources under `$GOPATH/src` | yaegi README | official |
| Crowdsec keeps `New`/`CreateConfig` in the root package and helpers in `pkg/…` | crowdsec@ae7481ca | source |
| Traefik loads helper subpackages only via the root package’s imports | derived from middlewareyaegi.go + Yaegi GOPATH | inference |

No conflict. Official docs omit subpackages; they do not forbid them. Official “no Go modules” is about third-party module fetch, not own-module packages. Follow Traefik source for what this version loads.

## References

- https://plugins.traefik.io/create
- https://github.com/traefik/plugindemo/blob/44419f66fe21c51f4c94fd46f8e02b98e4fb3168/readme.md
- https://github.com/traefik/traefik/blob/14bc52dd1f1d1c08cedd1da531a527fc04d79c19/pkg/plugins/middlewareyaegi.go
- https://github.com/traefik/traefik/blob/14bc52dd1f1d1c08cedd1da531a527fc04d79c19/pkg/plugins/types.go
- https://github.com/traefik/traefik/blob/14bc52dd1f1d1c08cedd1da531a527fc04d79c19/pkg/plugins/plugins.go
- https://github.com/traefik/yaegi/blob/fcb76d1ece0c3edc2548c39aa5b170475d2261bb/interp/interp.go
- https://github.com/traefik/yaegi/blob/fcb76d1ece0c3edc2548c39aa5b170475d2261bb/README.md
- https://github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/tree/ae7481caa5a06e9fd31dcc5017f7dc0f9e2d00b3/pkg
- https://github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/blob/ae7481caa5a06e9fd31dcc5017f7dc0f9e2d00b3/bouncer.go
- https://github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/blob/ae7481caa5a06e9fd31dcc5017f7dc0f9e2d00b3/.traefik.yml
