# Nitpicks

1. [hard] Name for the scope — `pkg/reclaim/table_test.go:1032` — in `TestTable_BindWhileArmingReclaims` the two generation snapshots are named `armingGen` and `stillArmingGen`; `stillArmingGen` is read *after* `bindLocked` and the whole point of the assertion is that the generation moved, so the identifier names the opposite of its role and the reader has to run the body to tell the two apart

   ```go
   tab.mu.Lock()
   e.arming = true
   armingGen := e.graceGen
   _, reclaimed := tab.bindLocked(e)
   stillArmingGen := e.graceGen
   tab.mu.Unlock()
   ...
   if stillArmingGen == armingGen {
       t.Fatal("a bind inside the arming window must bump the grace generation so the arm is skipped")
   }
   ```

   → `genBeforeBind` / `genAfterBind` (the role each snapshot has in this body)
   Status: done
   Argument: Renamed to `genBeforeBind` / `genAfterBind`, including the assertion that compares them.

2. [hard] Name for the scope — `pkg/reclaim/table_test.go:1109` — `nilDoneCtx.end()` cancels a holder context, but `end`/`ended` is already this file's word for "the stored value's `Close()` ran" (`ending(n, &ended)`, `box.ended`). Both meanings sit in one body:

   ```go
   var ended atomic.Bool            // the value's Close ran
   holder := &nilDoneCtx{}
   ...
   if ended.Load() { ... }
   holder.end()                     // cancel the holder context
   waitUntil(t, ended.Load)
   ```

   The reader must decode which `end` is which on every line of `TestTable_HolderWithoutDoneChannelIsPolled`. Every other holder in this file is retired with `cancel()`.

   → Rename the method to `cancel()` (and its backing field to `canceled`) so the holder role reads the same as every sibling context in the file
   Status: done
   Argument: `nilDoneCtx.end` is now `cancel`, the field is `canceled`, and the two doc comments that said "after end is called" follow. `ended` in that test body now means only what it means everywhere else in the file.

3. [hard] Name for the scope — `pkg/dbwrappers/reclaim_test.go:79` — the renamed helper `useLeases` drops the clause its callers actually need. The body resets the process table to `grace` *and* mints the `recHandler` that two of the four call sites go on to assert against; the name says neither, and `use` is the vague-verb family.

   ```go
   // useLeases resets the process table with grace and returns a handler recording its reclaim lines.
   func useLeases(t *testing.T, grace time.Duration) *recHandler {
       t.Helper()
       h := &recHandler{}
       ResetWith(grace)
       return h
   }
   ```

   → Spell the whole job, e.g. `resetTableWithGraceAndRecorder(t, grace)` (or `newLeaseRecorder(t, grace)`); the doc comment already states the job the identifier omits
   Status: done
   Argument: Renamed to `resetTableWithRecorder` at the definition and all four call sites. Took the shorter of the two suggestions: `grace` is already the parameter, so naming it again in the identifier repeats the signature.

4. [judgement] Clear conditions — `pkg/reclaim/table.go:178` — `e.graceTimer != nil || e.arming` is the unnamed concept "grace is already pending on this slot" (armed timer, or the window between the orphan line and the arm). It is spelled out in `bindLocked` and again inside `drop`'s guard at `table.go:216` (`len(e.holders) > 0 || e.graceTimer != nil || e.arming`), and both sites need a comment to say that the two fields are one state.

   ```go
   func (t *Table) bindLocked(e *slot) (id uint64, reclaimed bool) {
       if e.graceTimer != nil || e.arming {
           if e.graceTimer != nil {
               e.graceTimer.Stop()
               e.graceTimer = nil
           }
           e.graceGen++
           reclaimed = true
       }
   ```

   → Add a predicate named for the concept — `func (e *slot) gracePending() bool { return e.graceTimer != nil || e.arming }` — and call it at both sites (additive, stdlib-only, so it still ports as a file copy)
   Status: done
   Argument: Added `(*slot).gracePending` and called it in `bindLocked` and `drop`. The `bindLocked` doc comment now says "grace pending means the slot was orphaned" instead of enumerating the two fields.

5. [judgement] Name for the scope — `pkg/reclaim/table_test.go:1136` — `TestTable_ValueWithoutCloseDisposesQuietly` is named for a mood, and that mood contradicts what the body asserts: the test waits for the dispose line to be logged (`waitKeyMsg(t, h, MsgDispose, "a")`), so the disposal is not quiet. What "quietly" is standing in for is "without a `Close()` method and without panicking".

   → `TestTable_ValueWithoutCloseStillDisposes`
   Status: done
   Argument: Renamed to `TestTable_ValueWithoutCloseStillDisposes`.
