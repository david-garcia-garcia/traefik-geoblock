# Dead

1. [judgement] Dead branch or constant — `pkg/reclaim/table.go:268` — the `e.closed == nil` guard in `waitClosed` is newly introduced by this diff (049a130) and no code path can satisfy it: the only `*slot` literal in the package sets `closed: make(chan struct{})` unconditionally, and both callers (`fire`, `Reset`) only ever see slots taken from `t.items`, which is populated at that one site.
   Grep: `rg -n "slot\{|waitClosed|\.arming|\.closed\b|closed:|graceGen" --glob "*.go" pkg/reclaim pkg/dbwrappers plugin.go` — the only slot construction is `pkg/reclaim/table.go:153` (`&slot{ ... closed: make(chan struct{}) ... }`), plus `map[string]*slot{}` at lines 70 and 281; no test constructs a `slot` either, so nothing exercises the nil case.
   Hunk:
   ```
   func waitClosed(e *slot) {
       if e.closed == nil {
           return
       }
       <-e.closed
   }
   ```
   → Drop the nil check and let `waitClosed` be `<-e.closed`; if the defensive shape is kept deliberately for parity with the pre-existing (equally unreachable) `if cancel != nil` guard at `pkg/reclaim/table.go:258` and for the `traefik-modsecurity` port, say so in the comment instead of "A slot built without that goroutine has no channel", which describes a slot the package cannot build.
   Status: done
   Argument: Went further than the fix line: with the nil branch gone the helper was a one-line wrapper, so `waitClosed` is deleted and `fire` and `Reset` receive on `<-e.closed` directly. The `slot.closed` field comment now states the invariant that made the branch dead ("every slot must have one") instead of describing a slot the package cannot build. `go vet` clean, `-count=3` on `pkg/reclaim` and `pkg/dbwrappers` green.

2. [judgement] Test-only production symbol — `pkg/reclaim/table.go:275` — `(*Table).Reset` has no production caller in this repo. It is reached only through `reclaim.ResetWith` / `reclaim.Reset` (`pkg/reclaim/default.go:32,37`) and `dbwrappers.Reset` / `dbwrappers.ResetWith` (`pkg/dbwrappers/reset.go:10,15`), and every leaf caller of that chain is a `_test.go` file. Pre-existing, not introduced by this diff: 049a130 only edits the body (clears `arming`, calls `waitClosed`) and the doc comment. The identifier does not say test, but the doc comments already say "Tests only."
   Grep: `rg -n "reclaim\.Reset|reclaim\.ResetWith|ResetWith\(|NewTable\(|func Reset|dbwrappers\.Reset|Reset\(\)" --glob "*.go" .` — non-test hits are only the definitions themselves plus the forwarding wrappers; every call site is in `pkg/reclaim/table_test.go`, `pkg/dbwrappers/*_test.go`, `pkg/geoblock/*_test.go`, or `plugin_instance_test.go`. `NewTable` is not in this state — `Default()` calls it at `pkg/reclaim/default.go:20`.
   → No action. The human ruled `pkg/reclaim` is a copy shared with `traefik-modsecurity`, kept in sync in both directions, and that nothing may be removed — including code with no production caller in this repo; `Table.Reset` may well have a production caller there. The axis checklist also scopes this axis to same-change symbols, and this diff neither added `Reset` nor removed its callers. Recorded so a future reader does not re-derive it.
   Status: skipped
   Argument: The no-removal rule for this shared copy forbids acting on it, and the finding itself recommends no action. Pre-existing, outside this diff.
