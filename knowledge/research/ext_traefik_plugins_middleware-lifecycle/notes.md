# Plugin middleware lifecycle

Official Traefik Yaegi and local plugin middleware: when `New` runs, what a dynamic-config reload does, and whether Traefik tears the handler down. Not how this product wraps that.

Pinned implementation: [traefik@83c3499f](https://github.com/traefik/traefik/tree/83c3499fc31c96e9f80ea0bba4d975f608c7061d) (master at clone time). Local and catalog Yaegi plugins share this path; WASM and provider plugins do not.

## Official docs define New, not teardown

A Yaegi middleware plugin is a Go package that exports `Config`, `CreateConfig`, and `New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error)` ([plugindemo readme](https://github.com/traefik/plugindemo/blob/44419f66fe21c51f4c94fd46f8e02b98e4fb3168/readme.md)). The catalog create page says the same architecture and points at that repo ([Developing Traefik Plugins](https://plugins.traefik.io/create)). Neither page, nor [Extend Traefik](https://doc.traefik.io/traefik/master/extend/extend-traefik/), names `Close`, `Stop`, or a reload hook for middleware plugins.

Official loading rule: plugins are parsed and loaded **only at Traefik startup**. You cannot add or change plugin code while Traefik is running. After that, “middleware plugins behave exactly like statically compiled middlewares. Their instantiation and behavior are driven by the dynamic configuration” ([plugindemo readme](https://github.com/traefik/plugindemo/blob/44419f66fe21c51f4c94fd46f8e02b98e4fb3168/readme.md)).

Extracts: [.sources/plugindemo-readme.md](.sources/plugindemo-readme.md), [.sources/create.md](.sources/create.md), [.sources/extend-traefik.md](.sources/extend-traefik.md)

## Builder once at startup; New once per router that lists the middleware

`plugins.NewBuilder` constructs one Yaegi interpreter and one `yaegiMiddlewareBuilder` per plugin name from static config (catalog or `localPlugins`). That builder only `Eval`s `New` and `CreateConfig` ([traefik@83c3499f:pkg/plugins/builder.go](https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/plugins/builder.go), [cmd/traefik/plugins.go](https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/cmd/traefik/plugins.go)).

When a router is built, `BuildMiddlewareChain` walks that router’s middleware list. For a plugin it calls `pluginBuilder.Build` (`CreateConfig` + decode) and then the alice constructor, which calls `YaegiMiddleware.NewHandler` → plugin `New(ctx, next, config, middlewareName)` ([traefik@83c3499f:pkg/server/middleware/middlewares.go](https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/middleware/middlewares.go), [pkg/server/middleware/plugins.go](https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/middleware/plugins.go), [pkg/plugins/middlewareyaegi.go](https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/plugins/middlewareyaegi.go)).

`buildHTTPHandler` calls `BuildMiddlewareChain` for **each router** and `alice.Then` materializes the chain immediately ([traefik@83c3499f:pkg/server/router/router.go](https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/router/router.go)). There is no shared handler instance keyed by middleware name. Two routers that list the same plugin middleware each get their own `New` call. `New` is not per HTTP request.

Extracts: [.sources/builder.go.md](.sources/builder.go.md), [.sources/middlewareyaegi.go.md](.sources/middlewareyaegi.go.md), [.sources/middlewares.go.md](.sources/middlewares.go.md), [.sources/plugins.go.md](.sources/plugins.go.md), [.sources/router.go.md](.sources/router.go.md)

## Applied dynamic reload always rebuilds New, even if plugin config is unchanged

Every applied dynamic configuration runs `switchRouter` → `RouterFactory.CreateRouters` ([traefik@83c3499f:cmd/traefik/traefik.go](https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/cmd/traefik/traefik.go)). `CreateRouters` always:

1. Cancels the previous factory context (`cancelPrevState`).
2. Makes a new `context.WithCancel`.
3. Builds a new middleware builder and router manager and calls `BuildHandlers` ([traefik@83c3499f:pkg/server/routerfactory.go](https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/routerfactory.go)).

There is no Yaegi cache of “same middleware name + same plugin config → reuse handler”. A change to any other router, service, or middleware in the applied config still rebuilds every plugin middleware, including ones whose own plugin block did not change.

The configuration watcher does **not** apply a provider message that is `reflect.DeepEqual` to that provider’s last config (`Skipping unchanged configuration`). The same skip exists on the merged set before listeners run ([traefik@83c3499f:pkg/server/configurationwatcher.go](https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/configurationwatcher.go)). So an identical file rewrite that produces a DeepEqual-equal tree does not call `New` again. Any applied difference does.

After rebuild, entrypoints swap the HTTP handlers (`Switch` / `UpdateHandler`) so the old chain is no longer served ([traefik@83c3499f:pkg/server/server_entrypoint_tcp.go](https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/server_entrypoint_tcp.go)). Removing a router is the same rebuild: that router is omitted from the new mux; there is no per-router destructor.

Extracts: [.sources/traefik.go.md](.sources/traefik.go.md), [.sources/routerfactory.go.md](.sources/routerfactory.go.md), [.sources/configurationwatcher.go.md](.sources/configurationwatcher.go.md), [.sources/server_entrypoint_tcp.go.md](.sources/server_entrypoint_tcp.go.md)

## No Close or Stop on Yaegi middleware; context cancel is the hook

The Yaegi middleware contract Traefik calls is `New` / `CreateConfig` / `NewHandler`. `pluginMiddleware` is only `NewHandler(ctx, next)` ([traefik@83c3499f:pkg/plugins/builder.go](https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/plugins/builder.go)). Traefik never type-asserts the returned `http.Handler` as `io.Closer` and never looks up a plugin `Close` or `Stop`.

**Provider** plugins are a different type and do export `Stop()` ([traefik@83c3499f:pkg/plugins/providers.go](https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/plugins/providers.go)). That does not apply to HTTP middleware plugins.

The `ctx` passed into plugin `New` is the `CreateRouters` cancel context. The next applied dynamic config cancels it **before** the next `New` runs. Traefik staff stated the same: in a Yaegi plugin, use the context received by `New`; it is `Done()` when a new dynamic configuration is deployed, before a new `New` ([traefik#10547](https://github.com/traefik/traefik/issues/10547), juliens, 2024-07-02). WASM does not get that cancel; the issue stays open for WASM.

That cancel behavior is present on this pin. A later comment on the same issue said v2.11.7 did not cancel; that is comment-rank and was not re-checked in this clone. Follow this pin for what current master does.

Measured on this product’s compose image `traefik:v3.7.11` (2026-08-28). Probe in plugin `New` logged `ctx` as `*context.valueCtx`, `Err` nil, no deadline, `Cause` nil. A goroutine blocked on `ctx.Done()`. Adding then removing an unrelated Docker router (plugin YAML unchanged) produced: both `geoblock` and `geoblock2` `ctx.Done` with `Err`/`Cause` `context canceled`, then a new `New` each (~1 ms later). Same order on the second reload. This `ctx` is the router-generation teardown signal, not the HTTP request context (`ServeHTTP` uses `req.Context()`). It is not `Close`/`Stop`. A naive `Stop` of a process-wide singleton on this cancel races the next `New` that reuses the same singleton.

Extracts: [.sources/builder.go.md](.sources/builder.go.md), [.sources/providers.go.md](.sources/providers.go.md), [.sources/routerfactory.go.md](.sources/routerfactory.go.md), [.sources/issue-10547.md](.sources/issue-10547.md), [.sources/compose-v3.7.11-lifecycle-probe.md](.sources/compose-v3.7.11-lifecycle-probe.md)

## Goroutines started in New leak unless they watch ctx

Traefik does not track or stop goroutines the plugin starts. The WASM builder comment states the host fact: “Traefik does not Close the middleware when creating a new instance on a configuration change” ([traefik@83c3499f:pkg/plugins/middlewarewasm.go](https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/plugins/middlewarewasm.go)). WASM then uses an internal `mw.Close` finalizer; Yaegi has no equivalent.

Inference from the files above: a goroutine started in `New` that does not select on `ctx.Done()` keeps running after reload (old and new both live). One that exits on `ctx.Done()` stops when the next applied dynamic config cancels the previous factory context. Issue #10547 exists because plugins that ignored this leaked work across reloads.

Extracts: [.sources/middlewarewasm.go.md](.sources/middlewarewasm.go.md), [.sources/issue-10547.md](.sources/issue-10547.md)

## Authority

| Claim | Owner | Rank |
| --- | --- | --- |
| Plugin exports `Config`, `CreateConfig`, `New(ctx, next, config, name)` | plugindemo README | official |
| Plugins loaded only at startup; instantiation driven by dynamic config | plugindemo README | official |
| Create page does not define Close / reload | https://plugins.traefik.io/create | official |
| Extend Traefik page does not define middleware teardown | https://doc.traefik.io/traefik/master/extend/extend-traefik/ | official |
| One Yaegi builder per plugin name at startup | traefik@83c3499f:builder.go | source |
| `New` is invoked as `(ctx, next, config, middlewareName)` and must return `http.Handler` | traefik@83c3499f:middlewareyaegi.go | source |
| Plugin interface is `NewHandler` only | traefik@83c3499f:builder.go | source |
| Each router that lists the middleware materializes its own chain (`New` per router) | traefik@83c3499f:router.go + middlewares.go | source |
| Applied dynamic config cancels previous ctx then rebuilds all handlers | traefik@83c3499f:routerfactory.go | source |
| Watcher skips DeepEqual-unchanged provider / merged config | traefik@83c3499f:configurationwatcher.go | source |
| Entrypoints swap handlers after rebuild | traefik@83c3499f:server_entrypoint_tcp.go | source |
| Provider plugins have `Stop()`; middleware plugins do not | traefik@83c3499f:providers.go | source |
| Host does not Close middleware on config change (WASM comment) | traefik@83c3499f:middlewarewasm.go | source |
| Yaegi `New` ctx is `Done()` on new dynamic config, before next `New` | traefik#10547 (juliens) | vendor |
| Same cancel-then-New on `traefik:v3.7.11`; `Err` is `context canceled`; no deadline; plugin YAML need not change | compose probe 2026-08-28 | ticket |
| Goroutines that ignore ctx leak across reload | derived from routerfactory cancel + no Close | inference |

No official-vs-source conflict. Official docs omit teardown; source and staff fill it. Follow source for this pin.

## References

- https://plugins.traefik.io/create
- https://doc.traefik.io/traefik/master/extend/extend-traefik/
- https://github.com/traefik/plugindemo/blob/44419f66fe21c51f4c94fd46f8e02b98e4fb3168/readme.md
- https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/plugins/middlewareyaegi.go
- https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/plugins/builder.go
- https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/plugins/providers.go
- https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/plugins/middlewarewasm.go
- https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/middleware/middlewares.go
- https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/middleware/plugins.go
- https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/router/router.go
- https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/routerfactory.go
- https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/configurationwatcher.go
- https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/server/server_entrypoint_tcp.go
- https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/cmd/traefik/traefik.go
- https://github.com/traefik/traefik/issues/10547
