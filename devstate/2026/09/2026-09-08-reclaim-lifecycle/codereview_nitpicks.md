# Nitpicks

1. [hard] Symmetry and consistency — `pkg/reclaim/table.go:193` — the two `Open` branches hand the slot to a transition helper with opposite lock protocols and no naming signal: the key-absent branch does `t.mu.Unlock()` and then `return t.put(ctx, key, s, logger, create)`, while `case slotAsleep: return t.reclaim(ctx, key, s, logger), nil` is reached with `t.mu` still held and `reclaim` (`table.go:248`) unlocks on the caller's behalf, so that branch of `Open` shows a `Lock` with no matching `Unlock`. `put`, `drop`, and `expire` all take the mutex themselves; the convention this diff removed (`bindLocked`) was the signal for "caller holds t.mu".
   → Rename to `reclaimLocked` so the name carries the protocol the two branches do not share (doc comment stays; behaviour unchanged).
   Status: open
   Argument: none.
2. [hard] Name for the scope — `pkg/reclaim/table_test.go:1048` — `ctxs := make([]context.CancelFunc, openers)` is a slice of cancel functions, not contexts; the body then reads `for _, cancel := range ctxs { cancel() }`. The predecessor had both `ctxs []context.Context` and `cancels []context.CancelFunc`; this change dropped the contexts slice and left the wrong stem on the survivor, while the sibling tests in the same file (`TestTable_ConcurrentCancelLastHolders`, `TestTable_ManyKeysDisposeIndependently`, `TestTable_GoroutinesReturnToBaseline`) all name this same role `cancels`.
   → Rename `ctxs` to `cancels`.
   Status: open
   Argument: none.
3. [hard] Name for the scope — `pkg/reclaim/table.go:173` — `s` is a letter-for-type placeholder for the incarnation, carried through every body this change rewrote (`Open`, `put`, `reclaim`, `watch`, `drop`, `expire`, `Reset`, plus the new test readers `readState`/`holderCount`). In `drop` it sits beside `value`, `logger`, `woken`, `mapped`, and `grace`, which are all named for their role, so it is the one local that does not say what it is; the type comment calls it "one incarnation".
   → Rename to `incarnation` (the identifier `slot` is taken by the type that `Open` constructs).
   Status: open
   Argument: none.
4. [hard] Name for the scope — `pkg/dbsource/updater_test.go:102` — the local `stopped := make(chan struct{})` (closed when `updater.Stop()` returns) shadows `stopped(stop <-chan struct{}) bool`, the package predicate this same change added in `updater.go:130`, so one identifier means "Stop has returned" in this body and "the loop was asked to end" one file over.
   → Rename the channel to `stopReturned`.
   Status: open
   Argument: none.
