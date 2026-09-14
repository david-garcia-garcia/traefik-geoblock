---
url: https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/28da9ab0c4c1ec8fdfc98366bbd18dcfcb160d8b/openspec/specs/std_go_reclaim_context-lease/spec.md
title: std_go_reclaim_context-lease spec
fetched: 2026-09-14
authority: source
ref: github.com/david-garcia-garcia/traefik-middleware-utilities@28da9ab0c4c1ec8fdfc98366bbd18dcfcb160d8b:openspec/specs/std_go_reclaim_context-lease/spec.md
---

Keyed reclaim table stores one any per key. Survives context cancel when the same key is opened again within grace. Table lives in reclaim. Not generic (Yaegi). table.go stdlib-only.

Caller constructs and owns a table. No process-wide table. New(Config) copies Grace; grace must not change after New. Two tables do not share even with the same key.

New(Config) is the only public constructor. Negative Grace → DefaultGrace 10s. Zero stays zero. NewTable, Default, package Open, package Reset, ResetWith SHALL NOT exist.

Open(ctx, key, logger, create, hooks): create once, store hooks, bind ctx. Panic if ctx nil. Error if logger nil. create takes no args. Register key before create. Create error: key not stored, waiters get the error, later Open retries. (value, nil) means live bind. ctx.Err() at bind → (nil, ctx.Err()), drop holder, Close MAY have run. Reclaim still waits for wake before return.

Nil-Done holder (Background) stays live until Err is set.

Grace is how long a sleeping value is kept. Zero: sleep and dispose with no wake window. Negative → 10s.

Logs: five debug msgs (put, bind, orphan, reclaim, dispose) plus reclaim_hook_panic at error. Lines mean the work already happened. Log not while mutex held. Orphan precedes dispose when Sleep returned.

Close hook at most once. Close panic recovered. AfterFunc holders do not park a waiter. Nil-Done watcher exits when incarnation ends (finished). EnforceCloseBeforeOpen keeps key mapped until Close returns; default unmaps first. Reset unmaps first regardless. Close window is not a sleeping window.

Create panic wrapped as reclaim: create %q: panic: %v. Nil create: reclaim: create %q: nil create. No Close in either case.

Bind-time finished snapshot under mutex. Every lock-held region uses defer. Zero-value Table{} Open returns error.

Library Open must load under Traefik Yaegi with caller-owned New(Config) table.
