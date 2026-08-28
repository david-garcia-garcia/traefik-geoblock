# Reclaim

## Language

**Reclaim set**:
A keyed table of incarnations. Each incarnation has one dispose callback and a set of holder contexts. The incarnation stays if a new context binds the same key before grace ends; otherwise dispose runs.
_Avoid_: factory, singleton map of `any`, wrapper, Traefik `Close`

**Put**:
Register the single dispose for a new incarnation. The creator calls this, not each holder.
_Avoid_: passing dispose on every Bind

**Bind**:
Attach one live holder context to a key. No dispose.
_Avoid_: refcount without storing contexts

**Grace**:
Wait after the last bound context for a key is Done, before dispose. A Bind in that window is a reclaim.
_Avoid_: dispose on `ctx.Done` with no wait

## Overview

Stdlib-only. Copy `pkg/reclaim` into another module. The caller keeps the typed object; this package never stores it. Many holders, one dispose.

## How to use

- `s := reclaim.NewSet(10 * time.Second, logger)` (or a short grace in tests).
- Watch stable `msg` + `key`. Info: `reclaim_put`, `reclaim_dispose`. Debug: `reclaim_bind`, `reclaim_orphan`, `reclaim_reclaim`.
- When you **create** the object: `s.Put(key, dispose)`.
- On every holder (including the creator and a grace reclaim): `s.Bind(key, ctx)`.
- `ctx` is the host teardown context (Traefik `New` ctx), not `req.Context()`.
- `dispose` stops background work and closes the object. The set runs it once per incarnation. Holders do not own it.
- Do not put the object in this package as `any`.

## Pattern snippet

```go
s := reclaim.NewSet(10 * time.Second, logger)
obj, created, err := getOrCreate(key)
if created {
	s.Put(key, func() { obj.Stop(); delete(byKey, key) })
}
s.Bind(key, ctx)
```

## Key files

- `pkg/reclaim` — `Set`, `Put`, `Bind`
- `openspec/specs` leaf `std_go_reclaim_context-lease` (after archive)

## Gotchas

- Hosts that cancel before they call the constructor again need grace (Traefik: ~1 ms, then `New`).
- Yaegi: keep typed maps in the caller package.
- A second `Put` while the incarnation is live or in grace does not replace dispose.
- Tests assert the `msg` constants, not Traefik compose. A config change is two keys: cancel A, Put+Bind B, wait grace, expect `reclaim_dispose` A.
