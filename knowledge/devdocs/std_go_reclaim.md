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

**Incarnation**:
One instance of a value under a key: it begins at the `Open` that found the key absent and ran `create`, and ends when its lifetime is canceled. A reclaim keeps the same incarnation, so the caller gets the same pointer; a create after dispose is a different one. The log lines carry the key, not an instance id, so two incarnations of a key are told apart by the `reclaim_put` between them.
_Avoid_: "the entry" or "the key" when you mean this instance; treating a reclaimed value as a new one

**Grace**:
Wait after the last bound context for a key is Done, before the incarnation lifetime is canceled. An `Open` in that window is a reclaim. Zero grace means no wait. Negative grace is `DefaultGrace` (10s).
_Avoid_: passing `0` when you meant the product default

**Lifetime**:
The per-incarnation context the table holds. It is canceled when the incarnation ends (grace elapsed while orphaned, `Reset`, or a lost create race); a goroutine started by `Open` watches it and calls `Close()` on the value. `create` takes no arguments, so callers never see this context.
_Avoid_: a house `dispose func(any)` on `Open`

## Overview

`pkg/reclaim` is reusable across packages. Yaegi panics on `reclaim.Table[*BIN]`; it loads a non-generic table of `any` and a type-assert in the caller.

**This package is a shared copy.** The same `pkg/reclaim` lives in [traefik-modsecurity](https://github.com/david-garcia-garcia/traefik-modsecurity/tree/main/pkg/reclaim) and the two are kept in sync in both directions: a change made here is ported there, and a change made there is ported here. So edits are **additive** — fix a defect, add coverage — and never remove or reshape what the package already does, even when a piece looks unreachable from this repo. Keep it stdlib-only and self-contained so the port stays a file copy. Before changing it, diff against upstream `main`.

## How to use

- Production: `reclaim.Open(ctx, key, logger, create)` (process table). Tests: `NewTable` with a short grace, or `ResetWith`. `logger` is required. `Reset` and `ResetWith` block until every stored value's `Close()` has returned, so a test can use them as teardown that really stops the tickers it started.
- Watch stable `msg` + `key`. All five (`reclaim_put`, `reclaim_bind`, `reclaim_orphan`, `reclaim_reclaim`, `reclaim_dispose`) are debug. Put/bind/reclaim use that `Open`’s logger; orphan/dispose use the last `Open` on the key. A middleware `logLevel` of info hides them.
- `ctx` is the host teardown context (Traefik `New` ctx), not `req.Context()`, not `context.Background()`.
- Give the stored value a `Close()` method if it must stop when the incarnation ends. The table calls it once, and waits for it before logging `reclaim_dispose` — so that line means the value is already stopped. `Close()` must not block: it holds up the grace timer (or `Reset`). `create` takes no arguments.
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
- A flag set inside the value's `Close()` is **not** a signal that `reclaim_dispose` was logged — `Close()` runs first, then the line. A test that waits on such a flag and then reads the log races it. Wait for the line (`waitKeyMsg`) and assert the flag afterwards.
- Canceling a holder and immediately calling `Open` again is usually *not* a reclaim: the drop runs on the holder's watcher goroutine, so the second `Open` often arrives while the first holder is still counted and is a plain second bind. A test that means to exercise the reclaim branch must wait for `reclaim_orphan` first.
- A test must never need a specific timer to win. Either use a grace long enough that the branch you assert cannot lose (`graceNoRace`), or accept both sides of the grace edge and assert what must hold for the side that happened. A test that requires "the reclaim beat the 3 ms timer" is a CI flake, not a check.
- `reclaim_orphan` is emitted before grace is armed, so it always precedes the `reclaim_dispose` that ends grace, at any grace. The `arming` window on the slot is what buys that ordering without logging under the table mutex. Two cases sit outside it by design: `Reset` cancels everything at once and can log dispose before an orphan line a concurrent cancel is still writing, and an `Open` that binds inside the arming window can record its `reclaim_reclaim` before that line.
