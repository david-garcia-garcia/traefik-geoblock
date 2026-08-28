---
url: https://github.com/traefik/traefik/blob/83c3499fc31c96e9f80ea0bba4d975f608c7061d/pkg/plugins/middlewarewasm.go
title: pkg/plugins/middlewarewasm.go
fetched: 2026-08-28
authority: source
ref: github.com/traefik/traefik@83c3499fc31c96e9f80ea0bba4d975f608c7061d:pkg/plugins/middlewarewasm.go
---

Comment in the WASM builder:

“Traefik does not Close the middleware when creating a new instance on a configuration change.”

WASM then sets `runtime.SetFinalizer` to call `mw.Close(ctx)` when the handler is GC’d. That `Close` is Traefik’s WASM host cleanup, not a Yaegi plugin export.
