---
url: https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/28da9ab0c4c1ec8fdfc98366bbd18dcfcb160d8b/reclaim/table.go
title: reclaim/table.go at v1.0.1
fetched: 2026-09-14
authority: source
ref: github.com/david-garcia-garcia/traefik-middleware-utilities@28da9ab0c4c1ec8fdfc98366bbd18dcfcb160d8b:reclaim/table.go
---

Only non-test source in package `reclaim`. Stdlib imports: context, fmt, log/slog, sync, time.

Constants: DefaultGrace = 10s; MsgPut, MsgBind, MsgOrphan, MsgReclaim, MsgDispose, MsgHookPanic.

Exported types: Hooks {Sleep, Wake, Close func(); EnforceCloseBeforeOpen bool}; Config {Grace time.Duration}; Table {unexported mu, grace, items}.

New(cfg Config) *Table copies Grace; negative Grace becomes DefaultGrace; zero stays zero.

Open(ctx, key, logger, create func() (any, error), hooks Hooks) (any, error). create takes no args (Yaegi). Nil table / nil logger / nil create / uninitialized items map → error. Nil ctx → panic. Hooks stored at put; later Open ignores hooks. Sleeping value is woken before Open returns. (value, nil) = live bind; ctx.Err() at bind → (nil, err) and drop.

Slot states: slotBusy, slotAwake, slotAsleep, slotGone. First Open registers busy before create. Concurrent waiters park on ready.

runHook recovers panics. dispose runs Close then Debug MsgDispose. Sleep/Close panics log Error MsgHookPanic.

Grace expire: time.AfterFunc (not go+select). Yaegi v0.16.1 interp._select can miss timer. Wait is not on the drop caller.

Holders with Done: context.AfterFunc. Nil Done: poll Err vs finished (20ms ticker). finished snapshot under mutex.

EnforceCloseBeforeOpen true: key stays mapped until Close returns. False: unmap then Close.

Reset: tests only; unmaps first regardless of EnforceCloseBeforeOpen; must not race Open. Awake: Sleep then Close. Sleep panic on Reset skips orphan.

Every lock region uses defer unlock.
