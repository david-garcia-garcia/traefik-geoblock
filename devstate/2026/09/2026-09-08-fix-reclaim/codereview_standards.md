# Standards

1. [hard] Name for the scope — `pkg/reclaim/table.go:54` — the new `slot` field `closed` names the slot, not the fact it carries. The fact is "the stored value's `Close()` has returned"; `e.closed` next to `value`, `cancel`, and `holders` reads as "this slot is closed / off the map", which is the *other* end-of-incarnation fact (`delete(t.items, key)`) — the exact conflation this change exists to separate. The field comment carries the clause the identifier drops.
   Hunk:
   ```
   	// closed is closed by the lifetime goroutine once it has run Close on the value.
   	// fire and Reset receive on it before they log dispose, so every slot must have one.
   	closed chan struct{}
   ```
   → Rename the field to `valueClosed` (`close(e.valueClosed)` in the lifetime goroutine, `<-e.valueClosed` in `fire` and `Reset`). Sibling `arming` needs no change: it sits directly under `graceTimer`, which is the only thing this scope arms.
   Status: open
   Argument: none.

2. [hard] Symmetry and consistency — `pkg/reclaim/default.go:26` — `Table.Open` and `Table.Reset` gained the tightened contract in their doc comments; the process-table façade in the sibling file still states the pre-change one, and the façade is what every production caller reads (`plugin.go`, `pkg/dbwrappers/reset.go` call `reclaim.Open` / `reclaim.Reset`, never the `Table` methods). `Open` at `:26` still ends at "calls it when the incarnation ends" without "before it logs dispose"; `Reset` at `:31` and `ResetWith` at `:36` still say only "cancels every lifetime" / "after canceling the current one", so nothing tells a caller that these now block until every value's `Close()` has returned — the one new cost the design accepted.
   Hunk:
   ```
   // Open is Default().Open: create-once for key on the process table and bind ctx.
   // logger is required. If the value has Close(), the table calls it when the incarnation ends.
   ...
   // Reset tears down the process table (cancels every lifetime) and installs a fresh one. Tests only.
   ...
   // ResetWith replaces the process table after canceling the current one. Tests only.
   ```
   → Carry the same clause onto the three façade comments: `Open` — "…calls it when the incarnation ends, before it logs dispose"; `Reset` / `ResetWith` — say they block until each value's `Close()` has returned. Additive, so the port to `traefik-modsecurity` stays a file copy.
   Status: open
   Argument: none.

3. [hard] Symmetry and consistency — `pkg/dbwrappers/reclaim_test.go:73` — this change introduces the same three test-support roles twice, spanning the two test files, spelled differently each time. Grace that the asserted branch cannot lose: `graceReclaimSafe` here (`:73`, 5 s) vs `graceNoRace` in `pkg/reclaim/table_test.go:23` (5 s, same comment intent) — and `graceNoRace` is the name the usage packet publishes (`std_go_reclaim`, Gotchas). Wait-for-a-reclaim-line: `waitEvent` here (`:87`) vs `waitKeyMsg` at `pkg/reclaim/table_test.go:146`. The budget: an inline `10 * time.Second` here (`:89`) vs the named `waitBudget` const at `pkg/reclaim/table_test.go:20`, whose comment says the same thing this one's does.
   Hunk:
   ```
   // graceReclaimSafe is long enough that a test asserting the reclaim branch cannot lose the grace timer.
   const graceReclaimSafe = 5 * time.Second
   ...
   func waitEvent(t *testing.T, h *recHandler, msg, key string) {
   	deadline := time.Now().Add(10 * time.Second)
   ```
   → In `pkg/dbwrappers/reclaim_test.go`, spell the roles the way `pkg/reclaim` already does: `graceNoRace`, `waitKeyMsg`, and a `waitBudget` const instead of the inline `10 * time.Second`. The identifiers cannot be shared across packages, but the same role must read the same in both files.
   Status: open
   Argument: none.

4. [hard] Leave a trail — `pkg/reclaim/table.go:236` — `drop`'s rewrite replaced the one block intro that named both end-of-holder outcomes ("Last holder gone: arm grace or end immediately when grace is zero") with a comment about the orphan window only, and the two blocks it used to cover now have no intro. Nothing in the body tells the reader that zero grace ends the incarnation inline on the watcher goroutine — and with `fire` now blocking on the value's `Close()`, that is precisely the fact the next reader needs. The design doc states it; the code does not.
   Hunk:
   ```
   	if t.grace == 0 {
   		t.mu.Unlock()
   		t.fire(key, e, gen)
   		return
   	}
   	e.graceTimer = time.AfterFunc(t.grace, func() { t.fire(key, e, gen) })
   	t.mu.Unlock()
   ```
   → Add the two block intros: one saying zero grace ends the incarnation on this goroutine (so `Close()` runs here), one saying grace is armed for this generation only.
   Status: open
   Argument: none.

5. [judgement] Duplicated Code — `pkg/reclaim/table_test.go:183` — the dispose-implies-closed assert is pasted verbatim at `:183`, `:326`, and `:772` (`waitKeyMsg` on `MsgDispose`, then `if !ended.Load()` with the identical `"dispose must not be logged before Close ran"` message), and `closeProbe` re-states the same contract twice more at `:967` and `:987`. Real but small cost of change: the contract sentence lives in five places, so re-wording it (or strengthening the check) means finding all of them by grepping the message string.
   Hunk:
   ```
   	waitKeyMsg(t, h, MsgDispose, "a")
   	if !ended.Load() {
   		t.Fatal("dispose must not be logged before Close ran")
   	}
   ```
   → Extract one helper next to `waitKeyMsg` (`requireDisposeAfterClose(t, h, key, ended)`) and call it from the three sites, so the contract wording lives once.
   Status: open
   Argument: none.
