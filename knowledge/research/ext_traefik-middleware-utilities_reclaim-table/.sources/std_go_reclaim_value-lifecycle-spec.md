---
url: https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/28da9ab0c4c1ec8fdfc98366bbd18dcfcb160d8b/openspec/specs/std_go_reclaim_value-lifecycle/spec.md
title: std_go_reclaim_value-lifecycle spec
fetched: 2026-09-14
authority: source
ref: github.com/david-garcia-garcia/traefik-middleware-utilities@28da9ab0c4c1ec8fdfc98366bbd18dcfcb160d8b:openspec/specs/std_go_reclaim_value-lifecycle/spec.md
---

Four events: create -> (sleep -> wake)* -> sleep -> close. create and close once. sleep/wake matched pair. No wake without sleep. Close always preceded by sleep.

Sleep keeps the value stored and its identity. Sleep panic: do not park asleep; Close the incarnation; order follows stored EnforceCloseBeforeOpen; no reclaim_orphan; still reclaim_dispose.

Open never returns a sleeping value. Wake has no error return / create-fallback. Wake panic: Open returns wrapping error, not the pointer; incarnation Closed; waiters get the same error. Later Open may create.

Positive-grace wait MUST be compiled time.AfterFunc (or equivalent). MUST NOT be interpreted go+select on timer and wake channel. Wait MUST NOT run on last-holder drop caller. Reclaim cancels the waiter.

sleep, wake, close are optional Hooks funcs on Open. Nil field skips. Table MUST NOT type-switch or assert the stored any for those events. EnforceCloseBeforeOpen is on that same Hooks value; read from stored hooks of the ending incarnation.

Hooks stored at put; later Open does not replace them.

Yaegi tests: type-switch on create any to Sleep does not match; Hooks funcs do run. Close panic recovered so AfterFunc cannot kill the process. Recovered panic surfaced once (error return or reclaim_hook_panic); never silent success.
