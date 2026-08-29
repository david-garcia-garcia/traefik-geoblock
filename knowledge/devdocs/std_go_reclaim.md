# Reclaim

## Language

**Table**:
A keyed store of `any` values plus holder contexts. The value stays if a new context opens the same key before grace ends; otherwise the incarnation lifetime is canceled. The caller type-asserts.
_Avoid_: `otherpkg.Table[*T]`, type alias/embed of that, Traefik `Close`

**Default**:
The process-wide table (`reclaim.Default`, `reclaim.Open`). One incarnation per key for the whole process.
_Avoid_: one `NewTable` per caller when they should share; unprefixed keys that can collide

**Open**:
Create-once for a key, then bind a holder context. Later `Open` does not run create.
_Avoid_: Put vs Bind as two public calls

**Grace**:
Wait after the last bound context for a key is Done, before the incarnation lifetime is canceled. An `Open` in that window is a reclaim.
_Avoid_: canceling the lifetime on `ctx.Done` with no wait

**Lifetime**:
The context `create` receives. It is canceled when the incarnation ends (grace elapsed while orphaned, `Reset`, or a lost create race).
_Avoid_: a house `dispose func(any)` on `Open`

## Overview

`pkg/reclaim` is reusable across packages. Yaegi panics on `reclaim.Table[*BIN]`; it loads a non-generic table of `any` and a type-assert in the caller.

## How to use

- Production: `reclaim.Open(ctx, key, create)` (process table). Tests: `NewTable` with a short grace, or `ResetWith`.
- Watch stable `msg` + `key`. Info: `reclaim_put`, `reclaim_dispose`. Debug: `reclaim_bind`, `reclaim_orphan`, `reclaim_reclaim`.
- `ctx` is the host teardown context (Traefik `New` ctx), not `req.Context()`.
- `create` receives `life`. Watch `life.Done()` to stop background work and close the value. The table cancels `life` once per incarnation.
- Prefix keys when more than one type shares Default (`bin:` / `mmdb:` / `plugin:`).

## Pattern snippet

```go
v, err := reclaim.Open(ctx, "bin:"+hash, func(life context.Context) (any, error) {
	w, err := newBIN(cfg)
	if err != nil {
		return nil, err
	}
	go func() {
		<-life.Done()
		w.close()
	}()
	return w, nil
})
w := v.(*BIN)
```

## Key files

- `pkg/reclaim/table.go` — `Table`, `Open`, logs
- `pkg/reclaim/default.go` — `Default`, package `Open`, `Reset`
- `openspec/specs/std_go_reclaim_context-lease/spec.md`

## Gotchas

- Hosts that cancel before they call the constructor again need grace (Traefik: ~1 ms, then `New`).
- Yaegi: do not write `Table[*T]` on a type from another package.
- A second `Open` while the incarnation is live or in grace does not replace the lifetime.
- Tests assert the `msg` constants. A config change is two keys: cancel A, Open B, wait grace, expect `reclaim_dispose` A.
