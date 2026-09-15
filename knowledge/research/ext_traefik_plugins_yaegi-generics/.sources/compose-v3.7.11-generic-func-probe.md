---
url: local compose traefik:v3.7.11 + throwaway plugin yaegi-generic-fn-probe
title: Yaegi v0.16.1 cross-package generic FUNCTION load probe
fetched: 2026-09-15
authority: ticket
---

Image: `traefik:v3.7.11`. Yaegi from Traefik: `github.com/traefik/yaegi@v0.16.1`. Throwaway
module `github.com/david-garcia-garcia/yaegi-generic-fn-probe` in `D:\tmp` (not committed),
loaded through `--experimental.localplugins`, dynamic config from a file provider so the probe
does not read the docker socket. `go vet ./...` clean before every load.

The root package must be named after the module (`yaegi_generic_fn_probe`), or load dies before
Yaegi evaluates anything: `failed to eval New: 1:28: undefined: yaegi_generic_fn_probe`.

Three packages, the real topology: `pkg/box` (stands in for `reclaim`) owns the generic helper,
`pkg/wrappers` (stands in for `dbwrappers`) owns the concrete `*BIN` and instantiates the helper
with it, the root package only calls `wrappers.OpenBIN`.

```go
// pkg/box: generic in T, and the factory returns the hooks with the value.
func OpenTypedWithHooks[T any](key string, create func() (any, Hooks, error)) (T, error)

// pkg/wrappers: instantiation happens here, next to the type.
func OpenBIN(key string, id int) (*BIN, error) {
	return box.OpenTypedWithHooks[*BIN](key, func() (any, box.Hooks, error) {
		created := &BIN{id: id}
		return created, box.Hooks{Sleep: created.sleep, Close: created.close}, nil
	})
}
```

Plugins stayed enabled (no `Plugins are disabled because an error has occurred`):

```
INF ... > Loading plugins... plugins=["fnprobe"]
DBG ... > FN-PROBE shapeA ok id= 7
DBG ... > FN-PROBE shapeA2 same= true  id= 7
DBG ... > FN-PROBE shapeB ok= true  id= 11
DBG ... > FN-PROBE hook=sleep id= 11
DBG ... > FN-PROBE shapeC ok id= 21  same= true
DBG ... > FN-PROBE wrappers hook=sleep id= 21
```

Shapes measured:

| Shape | What it is | Result |
|---|---|---|
| A | `box.OpenTyped[*db]` called in the root package, type arg from the root package, result in a local | PASS, typed return |
| A2 | Second call on the same key | PASS, `same=true`: one instance, no assert in the caller |
| B | Non-generic `OpenWithHooks`, factory returns `(any, Hooks, error)`, hooks are **method values** on the created pointer | PASS, stored Sleep fired later |
| C | `box.OpenTypedWithHooks[*BIN]` instantiated inside `pkg/wrappers` with that package's type, root calls only `wrappers.OpenBIN` | PASS, `same=true`, stored Sleep fired |

C is the shape that matters: the generic instantiation is a call expression in the package that
owns `T`, and no package-level declaration ever names `box.OpenTypedWithHooks[*BIN]`.
