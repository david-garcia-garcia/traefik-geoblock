# Reclaim

## Language

**Table**:
A keyed store of `any` values plus holder contexts. Sleep parks the value after the last holder; Wake returns it if Open happens before grace; Close ends the incarnation. `Table` itself is not generic; `OpenTyped` hands the value back already typed.
_Avoid_: `otherpkg.Table[*T]`, type alias/embed of that, Traefik `Close`, discovering methods on stored `any`

**Hooks**:
Optional `Sleep`, `Wake`, and `Close` funcs stored at first put, plus `EnforceCloseBeforeOpen`. A nil func skips that event. Later Open ignores hooks it did not create. `create` returns them beside the value, so they may be method values on what it just built.
_Avoid_: type-switch on create `any`; Close-only method discovery; a variable declared ahead of Open only so the hooks can close over it

**Open**:
Create-once for a key (`create` takes no args). Three entry points: `Open` takes hooks beside `create`, `OpenWithHooks` has `create` return them, and `OpenTyped[T]` does that with a typed return. A `*slog.Logger` is required and `ctx` must not be nil. Traefik’s `New` ctx is `WithCancel`; the next dynamic config cancels it before the next `New`.
_Avoid_: Put vs Bind as two public calls; a nil holder context; `func(context.Context) (any, error)` as create; process `Default`

**Grace**:
How long a **sleeping** value is kept before Close. An `Open` in that window Wakes the same pointer. Zero grace means Sleep then Close with no reclaim window. Negative grace is `DefaultGrace` (10s). Freeze at `New(Config)`.
_Avoid_: passing `0` when you meant the product default; treating grace as “how long the value stays live”

## Overview

The table is `github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim`, pinned in `go.mod` and loaded from `vendor/` (Yaegi does not fetch modules). Do not copy it into `pkg/reclaim`. `go mod vendor` omits `*_test.go`; library tests stay upstream. `Table` stays non-generic because Yaegi panics on `reclaim.Table[*BIN]`, but `OpenTyped[T]` is a generic **function**, which Yaegi runs as long as the instantiation stays a call expression. Each owner holds its own `*Table` from `New(Config)`.

## How to use

- Production: import `github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim`, then `reclaim.OpenTyped[*T](ctx, table, key, logger, create)` on a caller-owned table. Tests: `New(Config{Grace: short})` or `Reset` on that owner. `logger` is required. After a version bump, `go mod vendor` and re-apply `scripts/apply-oschwald-yaegi-patch.ps1`.
- Watch stable `msg` + `key`. All five (`reclaim_put`, `reclaim_bind`, `reclaim_orphan`, `reclaim_reclaim`, `reclaim_dispose`) are debug. Put/bind/reclaim use that `Open`’s logger; orphan/dispose use the last `Open` on the key. A middleware `logLevel` of info hides them.
- `ctx` is the host teardown context (Traefik `New` ctx), not `req.Context()`, not `context.Background()`.
- Return the `Hooks` from `create`, built on the value it just created. Sleep idle work (tickers) while parked; Wake it on reclaim; Close disposes.
- Prefix keys when more than one type could share a table (`bin:` / `mmdb:` / `plugin:`). This product uses two tables (plugin root, wrappers).

## Pattern snippet

```go
var table = reclaim.New(reclaim.Config{Grace: reclaim.DefaultGrace})

// OpenTyped is instantiated here, in the package that declares *BIN.
w, err := reclaim.OpenTyped[*BIN](ctx, table, "bin:"+hash, logger, func() (any, reclaim.Hooks, error) {
	created, err := newBIN(cfg)
	if err != nil {
		return nil, reclaim.Hooks{}, err
	}
	return created, reclaim.Hooks{
		Sleep: created.sleep,
		Wake:  created.wake,
		Close: created.close,
	}, nil
})
```

## Key files

- `vendor/github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim` — `Table`, `New`, `Open`, `OpenWithHooks`, `OpenTyped`, `Hooks`
- `plugin.go` — plugin-root table
- `pkg/dbwrappers` — wrappers table
- `openspec/specs/std_go_reclaim_context-lease/spec.md`

## Gotchas

- Hosts that cancel before they call the constructor again need a positive grace (Traefik: ~1 ms, then `New`).
- Do not copy the table into `pkg/reclaim`. Third-party code for Yaegi lives in `vendor/`. Library tests stay in utilities (`go mod vendor` skips `*_test.go`).
- Yaegi: do not write `Table[*T]` on a type from another package. Do not type-switch create `any` for Close/Sleep.
- Yaegi: `OpenTyped[*T]` must stay a call expression in a package that can name `T`. A package-level var, type alias, or struct field whose type names the instantiation fails to resolve.
- Wake does not run on first create. Start tickers inside `create`.
- The error path of `create` still returns a `Hooks` value; return the zero one, not hooks built on a value that does not exist.
- Tests assert the `msg` constants. A config change is two keys: cancel A, Open B, wait grace, expect `reclaim_dispose` A.
