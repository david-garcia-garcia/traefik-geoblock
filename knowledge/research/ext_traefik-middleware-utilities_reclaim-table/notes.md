# Reclaim table

Pinned source: [traefik-middleware-utilities](https://github.com/david-garcia-garcia/traefik-middleware-utilities) tag **v1.0.1**, commit `28da9ab0c4c1ec8fdfc98366bbd18dcfcb160d8b`. The published package is `github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim`. The only non-test implementation file is `reclaim/table.go` (stdlib imports only: `context`, `fmt`, `log/slog`, `sync`, `time`). Extracts: [`.sources/table.go.md`](.sources/table.go.md), [`.sources/README.md`](.sources/README.md), [`.sources/std_go_reclaim_context-lease-spec.md`](.sources/std_go_reclaim_context-lease-spec.md), [`.sources/std_go_reclaim_value-lifecycle-spec.md`](.sources/std_go_reclaim_value-lifecycle-spec.md), [`.sources/yaegi_test.go.md`](.sources/yaegi_test.go.md), [`.sources/std_go_reclaim.md`](.sources/std_go_reclaim.md).

This product vendors that module (`go.mod` + `vendor/`) and imports `…/reclaim`. Do not copy `table.go` or library tests into `pkg/reclaim`. The sections below are the v1.0.1 surface.

## Public API

Exported names on the library (not `_test.go`):

| Name | Kind |
|---|---|
| `DefaultGrace` | `10 * time.Second` |
| `MsgPut`, `MsgBind`, `MsgOrphan`, `MsgReclaim`, `MsgDispose`, `MsgHookPanic` | log `msg` constants |
| `Hooks` | struct: `Sleep`, `Wake`, `Close` `func()`, `EnforceCloseBeforeOpen bool` |
| `Config` | struct: `Grace time.Duration` |
| `Table` | keyed store |
| `New(cfg Config) *Table` | only public constructor |
| `(*Table) Open(...)` | create-once / bind / wake |
| `(*Table) Reset()` | tests only |

Source: [`reclaim/table.go`](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/28da9ab0c4c1ec8fdfc98366bbd18dcfcb160d8b/reclaim/table.go) at `28da9ab0`. Spec: `NewTable`, `Default`, package `Open`, package `Reset`, and `ResetWith` **shall not exist**. A test helper `NewTable` lives only in `reclaim/table_test.go` and is not part of the published surface.

## Types

`Table` is not generic. It stores `any`; callers type-assert. Yaegi panics or fails import on `otherpkg.Table[*T]`. Spec: [std_go_reclaim_context-lease](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/28da9ab0c4c1ec8fdfc98366bbd18dcfcb160d8b/openspec/specs/std_go_reclaim_context-lease/spec.md).

`Hooks` is the optional sleep / wake / close carrier plus `EnforceCloseBeforeOpen`. A nil func skips that event. The table stores the whole value at put and ignores a later `Open`’s `hooks` argument. Source: `Hooks` comment and `Open` godoc, `reclaim/table.go`.

`Config` is freeze-at-`New`. `New` copies `Grace`; later writes to that struct do not change the table.

Unexported: `slot` with states `slotBusy`, `slotAwake`, `slotAsleep`, `slotGone`. Not part of the public API.

## Open / create signatures

```go
func New(cfg Config) *Table
func (t *Table) Open(ctx context.Context, key string, logger *slog.Logger, create func() (any, error), hooks Hooks) (any, error)
```

- `create` takes **no arguments**. Yaegi cannot call `func(context.Context) (any, error)`.
- `logger` is required. Nil logger → error. The table has no logger of its own.
- Nil `create` → `fmt.Errorf("reclaim: create %q: nil create", key)` before the key is registered.
- Nil `ctx` → panic `"reclaim: Open requires a context"`.
- Nil `*Table` → error `"reclaim: open %q: nil table"`.
- Zero-value `Table{}` (nil `items`) → error `"reclaim: open %q: uninitialized table"`; does not panic. Construct with `New`.

`(value, nil)` means this call bound a holder that was still live at return. If `ctx.Err()` is set at bind, `Open` returns `(nil, ctx.Err())` and not the pointer, and drops that holder on the same call. Wake still runs to completion before that error return when reclaiming a sleeper.

The first `Open` for an absent key registers the key as `slotBusy` **before** `create` runs, so concurrent first Opens share one create. A create error (or recovered create panic, wrapped as `reclaim: create %q: panic: %v`) unmaps the key; waiters receive that error; a later Open may create again. Close is not called (nothing was stored).

Source: `Open`, `lookupOpen`, `put`, `finishBind` in `reclaim/table.go`; context-lease spec Open requirement.

## Grace

Grace is how long a **sleeping** value is kept before Close. It is not “how long the value stays live.”

- Negative `Config.Grace` → `DefaultGrace` (10s) at `New`.
- Zero grace → no sleeping window: last-holder drop runs Sleep then Close back to back; an Open racing that drop is a new put, not a reclaim.
- Positive grace → last-holder drop Sleeps, emits `reclaim_orphan`, then arms `time.AfterFunc`. An Open in that window is a reclaim: stop the timer, Wake, return the same pointer.

The waiter **must** be compiled stdlib `time.AfterFunc`. An interpreted `go` + `select` on `timer.C` and a wake channel can miss the timer under Yaegi v0.16.1 (`interp._select`). The wait does not run on the drop caller (a canceled bind must not stall Open for grace). Comment at `parkAsleep` in `reclaim/table.go`; test `TestYaegi_GraceExpireDoesNotHang` in `reclaim/yaegi_test.go`.

## Logging

Stable `msg` constants, all five lifecycle lines at **debug**:

| msg | when (after the named work) | logger |
|---|---|---|
| `reclaim_put` | successful create | this Open |
| `reclaim_bind` | holder attached | this Open |
| `reclaim_orphan` | Sleep returned (last holder gone) | last Open that bound |
| `reclaim_reclaim` | Wake returned | this Open |
| `reclaim_dispose` | Close returned (or nil Close, or recovered Close panic) | last Open that bound |

`reclaim_hook_panic` is **error** level: recovered Sleep or Close panic (no caller to answer). Attrs: `key`, `hook` (`sleep` / `close`), `panic`. Wake and create panics are returned as errors instead.

Put / bind / reclaim use that Open’s logger; orphan / dispose use the last Open that actually bound. Log lines do not run while `t.mu` is held. For one incarnation, `reclaim_orphan` precedes `reclaim_dispose` when Sleep returned. Sleep panic during `Reset` skips orphan and still disposes.

Source: constants and `dispose` / `drop` / `wakeAndBind` in `reclaim/table.go`; context-lease spec “Lifecycle events are logged”.

## Close / lifetime

There is **no** incarnation `context.Context` handed to `create`, and **no** `Close()` method discovery on the stored `any`.

Lifecycle on one stored value: `create -> (sleep -> wake)* -> sleep -> close`. Create and close run once. Sleep and wake are a matched pair. Close is always preceded by Sleep (including zero grace and `Reset` of an awake value). Sleep-panic paths skip orphan and still Close.

`Hooks.Close` (optional) runs once when the incarnation ends: grace elapsed, `Reset`, zero-grace drop, Sleep panic, or Wake panic. `reclaim_dispose` means Close has returned. Close panics are recovered (`runHook`); they cannot kill an AfterFunc goroutine.

`EnforceCloseBeforeOpen` (stored at put, default false):

- **false**: unmap first, then Close. A concurrent Open may create while Close is in flight.
- **true**: keep the key mapped `slotBusy` until Close returns, then create. Use when the value owns something exclusive (mmap, file lock, port, connection). Cost: a slow Close delays the next create (Traefik reload).

`Reset` is tests only and must not race `Open` on the same key. It unmaps first **regardless** of `EnforceCloseBeforeOpen`. Awake values are Slept then Closed; asleep values are Closed; busy/gone slots are left to the owning goroutine.

Holders with a Done channel use `context.AfterFunc` (no parked waiter per live hold). A holder whose `Done` is nil (`context.Background()`, Yaegi) is polled (`waitCtx`, 20ms ticker) until `Err` is set **or** the incarnation’s `finished` channel closes. The bind path snapshots `finished` under the mutex; if already nil, no watcher starts. Production should pass Traefik `New`’s `WithCancel` ctx, not Background.

Source: `Open` / `drop` / `expire` / `Reset` / `dropWhenDone` in `reclaim/table.go`; value-lifecycle spec.

## Default / process table

**Absent.** Callers construct with `New(Config)` and hold the `*Table` (typically package-scope in the plugin). Two tables never share an incarnation even with the same key. Independent keys on one table do not share an incarnation.

README pattern:

```go
var table = reclaim.New(reclaim.Config{Grace: reclaim.DefaultGrace})
stored, err := table.Open(ctx, "window:"+hash, logger, create, reclaim.Hooks{...})
```

Source: context-lease spec “Caller constructs and owns a table”; README “Reclaim table”; vendor usage doc `knowledge/devdocs/std_go_reclaim.md` in that repo.

## Yaegi constraints

Module `go 1.21`; Yaegi `v0.16.1` is a module require (interpreted tests). Constraints encoded in this package:

1. **Non-generic `any` table.** Do not write `Table[*T]` from another package.
2. **`create` has no args.** Yaegi cannot call `func(context.Context) (any, error)`.
3. **Hooks, not type-switch on create `any`.** Yaegi v0.16.1 synthesizes a create return with no methods; a type-switch to a Sleep/Close interface on that `any` does not match (`TestYaegi_CreateAnyTypeSwitchDoesNotMatch`). Pass `Hooks` funcs that close over a pointer assigned inside `create`.
4. **Grace expire is `time.AfterFunc`**, not interpreted `select` on a timer (see Grace).
5. **Nil-Done holders** are accepted (Yaegi `Background` shape) via Err polling; do not use that in production.
6. **Every lock-held region uses `defer` unlock**, so a panic recovered at the Yaegi plugin boundary cannot leave `t.mu` held.
7. **Hook and create panics are recovered** so AfterFunc / interpreted paths do not kill the process.
8. Yaegi tests that cancel contexts skip under `-race` (`Yaegi v0.16.1 select races inside the interp on context cancel`).
9. Hooks funcs **do** run under the interpreter (`TestYaegi_OpenHooksRunSleepWakeClose`).

Source: `reclaim/yaegi_test.go`, comments in `reclaim/table.go`, both specs, README (Yaegi subset).

## What is more flexible / robust than a typical in-tree copy

Relative to a copy that uses `Default` + `NewTable(grace)` + `Open(ctx, key, logger, create)` and type-asserts `Close()` on the stored value (`knowledge/devdocs/std_go_reclaim.md` in this product):

1. **Caller-owned tables** — no process singleton; tests use `New` + `Reset` on that instance; production holds one table per plugin package.
2. **Sleep / Wake** — a long grace is cheap because the value is asleep (idle resources released) rather than live. Traefik reload (~1 ms cancel then `New`) reclaims the same pointer instead of creating.
3. **Explicit `Hooks`** — works under Yaegi, where Close/Sleep method discovery on `func() (any, error)` does not. Callers close over the concrete pointer inside `create`.
4. **`EnforceCloseBeforeOpen`** — optional exclusive-resource serialization; default still allows overlap.
5. **Canceled bind does not return the pointer** — `(nil, ctx.Err())` instead of a value the caller must not use; last-holder canceled bind drops on that stack.
6. **Create-once under concurrency** — key registered busy before create; no discarded extra create (typical copy can create twice and `Close` the loser).
7. **Panic recovery** on create / Sleep / Wake / Close, plus `reclaim_hook_panic` so recovery is not silent.
8. **Nil-Done watcher lifetime** — `finished` channel so Background/Yaegi watchers exit when the incarnation ends (Reset / Close), instead of polling until process exit.
9. **Slot busy/ready protocol** — Open waits for in-flight create / Wake / Sleep / enforced Close instead of racing the map.
10. **Zero-value `Table{}` is an error**, not a panic on a nil map.

This plugin vendors third-party reclaim the same way it vendors ip2location and oschwald: `go.mod` require plus `go mod vendor`. Copying the package into `pkg/` is a fork, not a Traefik packaging requirement. `go mod vendor` omits `*_test.go`; library tests stay in the utilities repo.
