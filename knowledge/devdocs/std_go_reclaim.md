# Reclaim

## Language

**Reclaim set**:
A keyed table of dispose callbacks bound to a set of `context.Context` values. The instance stays if a new context binds the same key before grace ends; otherwise dispose runs.
_Avoid_: factory, singleton map of `any`, wrapper, Traefik `Close`

**Bind**:
Attach one live context to a key, with the dispose to run when that key is unreclaimed.
_Avoid_: refcount without storing contexts

**Grace**:
Wait after the last bound context for a key is Done, before dispose. A Bind in that window is a reclaim.
_Avoid_: dispose on `ctx.Done` with no wait

## Overview

Stdlib-only. Copy `pkg/reclaim` into another module. The caller keeps the typed object; this package never stores it.

## How to use

- `s := reclaim.NewSet(10 * time.Second)` (or a short grace in tests).
- After you create or reuse the object for `key`, `s.Bind(key, ctx, dispose)`.
- `ctx` is the host teardown context (Traefik `New` ctx), not `req.Context()`.
- `dispose` stops background work and closes the object. It runs at most once per incarnation.
- Do not put the object in this package as `any`.

## Pattern snippet

```go
s := reclaim.NewSet(10 * time.Second)
obj, err := getOrCreate(key)
s.Bind(key, ctx, func() { obj.Stop(); delete(byKey, key) })
```

## Key files

- `pkg/reclaim` — `Set`, `Bind`
- `openspec/specs` leaf `std_go_reclaim_context-lease` (after archive)

## Gotchas

- Hosts that cancel before they call the constructor again need grace (Traefik: ~1 ms, then `New`).
- Yaegi: keep typed maps in the caller package.
