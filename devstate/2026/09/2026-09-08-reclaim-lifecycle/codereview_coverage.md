# Test coverage

Ticket job (`requirement.md` Desired 1-5, `std_go_reclaim_value-lifecycle`): drive a stored value
through `create -> (sleep -> wake)* -> sleep -> close`, with `close` always preceded by `sleep` on
every ending path and no caller ever receiving a sleeper.

That job is proven, and each of these fails if its production hunk is reverted:
`pkg/reclaim/table_test.go:337` one `create`, alternating sleep/wake over five cycles, same
pointer, one `close` after `Reset`; `:383` sleep-then-close at zero and 5ms grace; `:402` `Open`
neither returns nor logs a bind while `Wake` blocks (and it waits for `reclaim_orphan` first, so
it is genuinely in the wake branch); `:447` one `wake` and one `reclaim` line for eight
concurrent openers; `:489` one `create` and exactly one created value for eight racing first
`Open`s, nothing discarded; `:545` the first create's error reaching all four parked callers,
nothing stored, retry allowed; `:735` `reclaim_orphan` before `reclaim_dispose` at 1ns grace over
200 rounds; `:753` and `:775` `reclaim_dispose` written only after `Close()` returned, on the
grace and the `Reset` path; `:985` and `:1019` `Reset` sleeps an awake incarnation and does not
re-sleep a sleeping one; `:952` and `:970` a value with no lifecycle methods and one with only
`Close`; `:1104` one `sleep` and one orphan for eight simultaneous last-holder cancels; `:1331`
goroutines back to baseline over 50 keys. `pkg/dbsource/updater_test.go:94` `Stop` does not
return during an in-flight download and nothing lands in the dir afterwards; `:129` and `:166` a
stopped `tick` performs no request or callback while a running one does (so `:129` is not
vacuous); `:191` stop-then-start runs one fresh loop and a second `Start` is ignored; `:213`
double stop, never-started, and nil are safe. `pkg/dbwrappers/reclaim_test.go:142` and `:201` BIN
and MMDB sleep/wake wiring — both wait for `reclaim_orphan`, assert the updater is gone while
asleep, a fresh loop plus a working lookup after wake, identity preserved, no dispose; `:260` a
disposed BIN holds no loop, answers no lookup, and asks the source no more.

1. [hard] Assertion does not prove the job — `pkg/reclaim/table_test.go:704` — `TestTable_ZeroGraceRacingOpenIsPlainBind` is the only test of the zero-grace unmap guard (`pkg/reclaim/table.go:311-315`) and it is a vacuous negative. It launches `cancel1()` and the second `Open` as two bare goroutines and asserts `countMsg(MsgReclaim) == 0`. The `Open` almost always wins that start, so it arrives while `holders == 1` and is a plain second bind; the sleeping window is never reached. With the guard deleted, `drop` would leave the key mapped in `slotAsleep` only for the ~microsecond between the unlock at `table.go:316` and `expire`'s lock (one slog line plus a timer alloc), which 60 unsynchronised rounds will not hit — the test stays green, so it proves nothing about zero grace logging `reclaim_put`/`reclaim_bind` instead of `reclaim_reclaim`. Stable at `-count=25` under `GOMAXPROCS=2`, consistent with the window never being entered. No other test covers the guard: `TestTable_ZeroGraceEndsImmediately:698` asserts `mappedKeys == 0`, which `expire` satisfies with or without it.
   → Make the race deterministic instead of hoping for it: give the stored value a blocking sleep hook (the `lifecycle` instrument at `table_test.go:68-74` already has `onWake` and `onClose`), start the second `Open` only once `Sleep()` has been entered so it parks on `slot.ready`, release the hook, then assert that `Open`'s key sequence is `reclaim_put` + `reclaim_bind` and never `reclaim_reclaim` — an assertion that flips the moment the zero-grace unmap is removed.
   Status: open
   Argument: none.
2. [judgement] Happy path only — `pkg/reclaim/table.go:195` — `Open`'s `slotBusy` wait names three owners ("a create, wake, or sleep owns the slot"); two are proven deterministically (`table_test.go:489` for create, `:447` for wake). The sleep-in-flight arm — an `Open` that arrives while `Sleep()` is running must wait and then receive an awake value — has no deterministic test: `TestTable_ReclaimRacesExpiry:1211` races it but accepts either outcome, and the `lifecycle` instrument has no sleep hook, so no test can hold an `Open` inside `Sleep()`. A caller handed a just-slept, never-woken value at non-zero grace would not be caught.
   → Reuse the sleep hook from finding 1 at non-zero grace: park an `Open` inside `Sleep()`, release it, then assert the value's event sequence ends in `wake` and that the `Open` logged `reclaim_reclaim` + `reclaim_bind`.
   Status: open
   Argument: none.
