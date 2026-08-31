# Reclaim

## Language

**Table**:
A keyed store of `any` values plus holder contexts. The value stays if a new context opens the same key before grace ends; otherwise the incarnation lifetime is canceled. The caller type-asserts.
_Avoid_: `otherpkg.Table[*T]`, type alias/embed of that, Traefik `Close`

**Default**:
The process-wide table (`reclaim.Default`, `reclaim.Open`). One incarnation per key for the whole process.
_Avoid_: one `NewTable` per caller when they should share; unprefixed keys that can collide

**Open**:
Create-once for a key (`create` takes no args — Yaegi assigns a `context.Context` arg onto the value). The caller passes a `*slog.Logger` (required; no table logger and no fallback). Later `Open` does not run create. If the value has `Close()`, the table calls it when the incarnation ends. `ctx` must not be nil. Traefik’s `New` ctx is `WithCancel`; the next dynamic config cancels it before the next `New`.
_Avoid_: Put vs Bind as two public calls; a nil holder context; `func(context.Context) (any, error)` as create

**Grace**:
Wait after the last bound context for a key is Done, before the incarnation lifetime is canceled. An `Open` in that window is a reclaim. Zero grace means no wait. Negative grace is `DefaultGrace` (10s).
_Avoid_: passing `0` when you meant the product default

**Lifetime**:
The context `create` receives. It is canceled when the incarnation ends (grace elapsed while orphaned, `Reset`, or a lost create race).
_Avoid_: a house `dispose func(any)` on `Open`

## Overview

`pkg/reclaim` is reusable across packages. Yaegi panics on `reclaim.Table[*BIN]`; it loads a non-generic table of `any` and a type-assert in the caller.

## How to use

- Production: `reclaim.Open(ctx, key, logger, create)` (process table). Tests: `NewTable` with a short grace, or `ResetWith`. `logger` is required.
- Watch stable `msg` + `key`. All five (`reclaim_put`, `reclaim_bind`, `reclaim_orphan`, `reclaim_reclaim`, `reclaim_dispose`) are debug. Put/bind/reclaim use that `Open`’s logger; orphan/dispose use the last `Open` on the key. A middleware `logLevel` of info hides them.
- `ctx` is the host teardown context (Traefik `New` ctx), not `req.Context()`, not `context.Background()`.
- Give the stored value a `Close()` method if it must stop when the incarnation ends. The table calls it once. `create` takes no arguments.
- Prefix keys when more than one type shares Default (`bin:` / `mmdb:` / `plugin:`).

## Pattern snippet

```go
v, err := reclaim.Open(ctx, "bin:"+hash, logger, func() (any, error) {
	return newBIN(cfg)
})
w := v.(*BIN) // *BIN has Close(); the table calls it when the incarnation ends
```

## Key files

- `pkg/reclaim/table.go` — `Table`, `Open`, logs
- `pkg/reclaim/default.go` — `Default`, package `Open`, `Reset`
- `openspec/specs/std_go_reclaim_context-lease/spec.md`

## Gotchas

- Hosts that cancel before they call the constructor again need a positive grace (Traefik: ~1 ms, then `New`). `NewTable(0)` ends the incarnation as soon as the last holder is gone.
- Yaegi: do not write `Table[*T]` on a type from another package.
- A second `Open` while the incarnation is live or in grace does not replace the lifetime.
- Tests assert the `msg` constants. A config change is two keys: cancel A, Open B, wait grace, expect `reclaim_dispose` A.
