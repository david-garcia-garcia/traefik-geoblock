# Reclaim

## Language

**Table**:
A keyed store of `any` values plus holder contexts. The value stays if a new context opens the same key before grace ends; otherwise it is closed and the key is dropped. The caller type-asserts.
_Avoid_: `otherpkg.Table[*T]`, type alias/embed of that, Traefik `Close`

**Default**:
The process-wide table (`reclaim.Default`, `reclaim.Open`). One incarnation per key for the whole process.
_Avoid_: one `NewTable` per caller when they should share; unprefixed keys that can collide

**Incarnation**:
One value from `create` to `close`, together with the holders bound to it. A key has at most one at a time. Sleeping and waking do not end an incarnation: a woken value is the same pointer the earlier holder had.
_Avoid_: calling a new value after a reclaim "the same incarnation"; it is not, and its `Close` runs separately

**Open**:
Create-once for a key (`create` takes no args — Yaegi cannot call `func(context.Context) (any, error)`). The caller passes a `*slog.Logger` (required; no table logger and no fallback). A later `Open` does not run create: it binds, and wakes the value first if it was asleep. `ctx` must not be nil. Traefik's `New` ctx is `WithCancel`; the next dynamic config cancels it before the next `New`.
_Avoid_: Put vs Bind as two public calls; a nil holder context; assuming `Open` returns fast when the value has a slow `Wake`

**Lifecycle**:
The four events the table drives on one stored value: `create -> (sleep -> wake)* -> sleep -> close`. `create` and `close` run once. `sleep` and `wake` are a matched, repeating pair. `close` is always preceded by `sleep`, on every ending path, so cleanup is never written twice.
_Avoid_: a house `dispose func(any)` on `Open`; cleanup in `Close` that `Sleep` already did

**Sleep**:
Optional `interface{ Sleep() }` on the stored value, called when the last holder's context is Done. The value stays stored and keeps its identity, and releases what is expensive to hold idle: an update ticker, an in-flight download. It is not a close: the value must be resumable.
_Avoid_: releasing something in `Sleep` that `Wake` cannot get back

**Wake**:
Optional `interface{ Wake() }`, called when an `Open` finds a stored, sleeping value. `Open` does not return until `Wake` has returned, so a caller never receives a sleeping value. `Wake` cannot fail: a value that cannot guarantee resume simply does not implement `Sleep`/`Wake`.
_Avoid_: an error return or a create-fallback on wake; work in `Wake` slow enough to stall a Traefik reload

**Grace**:
How long a **sleeping** value is kept before it is closed and its key dropped. An `Open` inside that window is a reclaim: it wakes the stored value instead of creating one. Zero grace keeps nothing — `sleep` and `close` run back to back and there is no window to be woken in. Negative grace is `DefaultGrace` (10s).
_Avoid_: passing `0` when you meant the product default; reading grace as "how long the value stays live" — it is asleep for all of it

## Overview

`pkg/reclaim` is reusable across packages. Yaegi panics on `reclaim.Table[*BIN]`; it loads a non-generic table of `any` and a type-assert in the caller.

Because grace now costs a sleeping value rather than a live one, a long grace is cheap. The reason to keep it long is no longer "reloads are fast", it is "a sleeping value is cheap to keep".

## How to use

- Production: `reclaim.Open(ctx, key, logger, create)` (process table). Tests: `NewTable` with a short grace, or `ResetWith`. `logger` is required.
- Watch stable `msg` + `key`. All five (`reclaim_put`, `reclaim_bind`, `reclaim_orphan`, `reclaim_reclaim`, `reclaim_dispose`) are debug. Put/bind/reclaim use that `Open`'s logger; orphan/dispose use the last `Open` on the key. A middleware `logLevel` of info hides them.
- `ctx` is the host teardown context (Traefik `New` ctx), not `req.Context()`, not `context.Background()`.
- Give the stored value `Sleep()` and `Wake()` to release and resume idle cost, and `Close()` for what only a final teardown should do. All three are optional and independent; a value with none of them still works.
- Write `Close` assuming `Sleep` already ran. That is the point of the split.
- Prefix keys when more than one type shares Default (`bin:` / `mmdb:` / `plugin:`).

## Pattern snippet

```go
v, err := reclaim.Open(ctx, "bin:"+hash, logger, func() (any, error) {
	return newBIN(cfg)
})
w := v.(*BIN) // *BIN has Sleep/Wake/Close; the table drives all three

// On the stored value: Sleep clears the updater under the wrapper lock and joins it outside,
// Wake cannot fail so it logs, and Close is left with only what Sleep does not touch.
func (w *BIN) Sleep() {
	w.mu.Lock()
	updater := w.updater
	w.updater = nil
	w.mu.Unlock()
	updater.Stop() // Stop does not return until the loop goroutine has finished
}

func (w *BIN) Wake() {
	if err := w.startUpdate(); err != nil {
		w.logger.Error("source updater", "error", err)
	}
}

func (w *BIN) Close() { /* release the database handle; Sleep already stopped the loop */ }
```

## Key files

- `pkg/reclaim/table.go` — `Table`, `Open`, the slot state machine, logs
- `pkg/reclaim/default.go` — `Default`, package `Open`, `Reset`
- `pkg/dbwrappers/bin.go`, `pkg/dbwrappers/mmdb.go` — the first values to implement sleep/wake
- `openspec/specs/std_go_reclaim_context-lease/spec.md`, `openspec/specs/std_go_reclaim_value-lifecycle/spec.md`

## Gotchas

- Hosts that cancel before they call the constructor again need a positive grace (Traefik: ~1 ms, then `New`). `NewTable(0)` ends the incarnation as soon as the last holder is gone.
- Yaegi: do not write `Table[*T]` on a type from another package, and do not give `create` an argument.
- A second `Open` while the incarnation is live or in grace returns the same value; it does not create a new one and it does not extend anything.
- Tests assert the `msg` constants. A config change is two keys: cancel A, Open B, wait grace, expect `reclaim_dispose` A.
- **A test that cancels a holder and immediately calls `Open` is usually not testing the wake branch.** The drop runs on the holder's watcher goroutine, so the second `Open` normally arrives while the first holder is still counted, and it is a plain second bind. Wait for the `reclaim_orphan` line first.
- `Open` blocks for as long as `Wake` takes, and a second `Open` on the same key blocks behind the first one's `create`. Both are deliberate — they are what stops a duplicated GeoIP download — but keep `Wake` cheap.
- At zero grace an `Open` that races the orphan log is a plain bind, not a reclaim. A zero-grace table has no grace window to reclaim into.
- `Reset` is tests only. It must not race an `Open` on the same key.
