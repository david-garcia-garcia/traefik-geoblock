# Reclaim

## Language

**Table**:
A keyed store of `any` values plus holder contexts. Sleep parks the value after the last holder; Wake returns it if Open happens before grace; Close ends the incarnation. The caller type-asserts.
_Avoid_: `otherpkg.Table[*T]`, type alias/embed of that, Traefik `Close`, discovering methods on stored `any`

**Hooks**:
Optional `Sleep`, `Wake`, and `Close` funcs stored at first put, plus `EnforceCloseBeforeOpen`. A nil func skips that event. Later Open ignores its hooks argument. Callers close over a pointer assigned inside `create` (Yaegi create returns have no methods).
_Avoid_: type-switch on create `any`; Close-only method discovery

**Open**:
Create-once for a key (`create` takes no args). The caller passes a `*slog.Logger` (required) and `Hooks`. `ctx` must not be nil. Traefik’s `New` ctx is `WithCancel`; the next dynamic config cancels it before the next `New`.
_Avoid_: Put vs Bind as two public calls; a nil holder context; `func(context.Context) (any, error)` as create; process `Default`

**Grace**:
How long a **sleeping** value is kept before Close. An `Open` in that window Wakes the same pointer. Zero grace means Sleep then Close with no reclaim window. Negative grace is `DefaultGrace` (10s). Freeze at `New(Config)`.
_Avoid_: passing `0` when you meant the product default; treating grace as “how long the value stays live”

## Overview

`pkg/reclaim` is reusable across packages. Yaegi panics on `reclaim.Table[*BIN]`; it loads a non-generic table of `any` and a type-assert in the caller. Each owner holds its own `*Table` from `New(Config)`.

## How to use

- Production: `table.Open(ctx, key, logger, create, hooks)` on a caller-owned table. Tests: `New(Config{Grace: short})` or `Reset` on that owner. `logger` is required.
- Watch stable `msg` + `key`. All five (`reclaim_put`, `reclaim_bind`, `reclaim_orphan`, `reclaim_reclaim`, `reclaim_dispose`) are debug. Put/bind/reclaim use that `Open`’s logger; orphan/dispose use the last `Open` on the key. A middleware `logLevel` of info hides them.
- `ctx` is the host teardown context (Traefik `New` ctx), not `req.Context()`, not `context.Background()`.
- Pass `Hooks` that close over the pointer assigned inside `create`. Sleep idle work (tickers) while parked; Wake it on reclaim; Close disposes.
- Prefix keys when more than one type could share a table (`bin:` / `mmdb:` / `plugin:`). This product uses two tables (plugin root, wrappers).

## Pattern snippet

```go
var table = reclaim.New(reclaim.Config{Grace: reclaim.DefaultGrace})
var w *BIN
v, err := table.Open(ctx, "bin:"+hash, logger, func() (any, error) {
	created, err := newBIN(cfg)
	if err != nil {
		return nil, err
	}
	w = created
	return created, nil
}, reclaim.Hooks{
	Sleep: func() { w.sleep() },
	Wake:  func() { w.wake() },
	Close: func() { w.close() },
})
typed := v.(*BIN)
```

## Key files

- `pkg/reclaim/table.go` — `Table`, `New`, `Open`, `Hooks`
- `plugin.go` — plugin-root table
- `pkg/dbwrappers` — wrappers table
- `openspec/specs/std_go_reclaim_context-lease/spec.md`

## Gotchas

- Hosts that cancel before they call the constructor again need a positive grace (Traefik: ~1 ms, then `New`).
- Yaegi: do not write `Table[*T]` on a type from another package. Do not type-switch create `any` for Close/Sleep.
- Wake does not run on first create. Start tickers inside `create`.
- Tests assert the `msg` constants. A config change is two keys: cancel A, Open B, wait grace, expect `reclaim_dispose` A.
