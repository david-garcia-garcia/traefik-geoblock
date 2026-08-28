# Reclaim

## Language

**Table**:
A keyed store of typed values plus holder contexts. The value stays if a new context opens the same key before grace ends; otherwise dispose runs.
_Avoid_: `otherpkg.Table[*T]`, type alias/embed of that, `any` box, Traefik `Close`

**Open**:
Create-once for a key, then bind a holder context. Later `Open` does not run create.
_Avoid_: Put vs Bind as two public calls

**Grace**:
Wait after the last bound context for a key is Done, before dispose. An `Open` in that window is a reclaim.
_Avoid_: dispose on `ctx.Done` with no wait

## Overview

`Table[T]` must be defined in the same package as `T`. Yaegi panics if you instantiate `reclaim.Table[*BIN]` from another package. Copy `pkg/dbwrappers/table.go` into the package that owns the value.

## How to use

- `tab := NewTable[*Thing](10 * time.Second, logger)` (short grace in tests).
- Watch stable `msg` + `key`. Info: `reclaim_put`, `reclaim_dispose`. Debug: `reclaim_bind`, `reclaim_orphan`, `reclaim_reclaim`.
- `tab.Open(ctx, key, create, dispose)` on every holder, including reclaim.
- `ctx` is the host teardown context (Traefik `New` ctx), not `req.Context()`.
- `dispose` stops background work and closes the value. The table runs it once per incarnation.

## Pattern snippet

```go
var things *Table[*Thing]

func OpenThing(ctx context.Context, key string) (*Thing, error) {
	if things == nil {
		things = NewTable[*Thing](10*time.Second, slog.Default())
	}
	return things.Open(ctx, key, newThing, func(v *Thing) { v.close() })
}
```

## Key files

- `pkg/dbwrappers/table.go` — `Table[T]`, `Open`, logs
- `openspec/specs` leaf `std_go_reclaim_context-lease` (after archive)

## Gotchas

- Hosts that cancel before they call the constructor again need grace (Traefik: ~1 ms, then `New`).
- Yaegi: do not import `Table` from another package and write `Table[*T]`.
- A second `Open` while the incarnation is live or in grace does not replace dispose.
- Tests assert the `msg` constants. A config change is two keys: cancel A, Open B, wait grace, expect `reclaim_dispose` A.
