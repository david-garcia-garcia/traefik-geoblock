---
url: https://github.com/traefik/traefik/blob/14bc52dd1f1d1c08cedd1da531a527fc04d79c19/pkg/plugins/middlewareyaegi.go
title: pkg/plugins/middlewareyaegi.go
fetched: 2026-08-26
authority: source
ref: github.com/traefik/traefik@14bc52dd1f1d1c08cedd1da531a527fc04d79c19:pkg/plugins/middlewareyaegi.go
---

`newInterpreter` constructs Yaegi with `interp.Options{GoPath: goPath, Env: os.Environ(), ...}`. It loads stdlib symbols (and optionally unsafe/syscall). It then:

```
i.Eval(fmt.Sprintf(`import "%s"`, manifest.Import))
```

Only `manifest.Import` is imported here. Failure is `failed to import plugin code %q`.

`newYaegiMiddlewareBuilder(i, basePkg, imp)`:

- If `basePkg == ""`, `basePkg = strings.ReplaceAll(path.Base(imp), "-", "_")`.
- `i.Eval(basePkg + ".New")`
- `i.Eval(basePkg + ".CreateConfig")`

`New` is called as `(ctx, next, config, middlewareName)` and must return an `http.Handler`. `CreateConfig` is called with no args and must return one value; Traefik then mapstructure-decodes the dynamic config into that value.

This file does not walk the plugin tree or import `pkg/…` itself.
