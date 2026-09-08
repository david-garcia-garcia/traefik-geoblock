# Explore

## Concepts

- **Incarnation** — one `slot` in `pkg/reclaim/table.go`: a stored value, its lifetime `cancel`, and the holder ids that still need it. Ends exactly once (`fire` deletes the key, or `Reset` swaps the map).
- **Grace edge** — the interval between the last holder's `drop` and the `AfterFunc` that calls `fire`. An `Open` inside it is a reclaim (`bindLocked` stops the timer and bumps `graceGen`); an `Open` after it hits a fresh incarnation. Both outcomes are legal; which one happens is a wall-clock race.
- **Dispose log vs Close** — `fire` calls `cancel()` then logs `reclaim_dispose`. On `origin/master` the stored value's `Close()` runs on a *separate* per-slot goroutine (`go func(){ waitCtx(life); stopValue(v) }()` at `pkg/reclaim/table.go:157-160`). So "dispose logged" does **not** imply "Close returned".
- **Upstream** — `david-garcia-garcia/traefik-modsecurity` `main` `pkg/reclaim/`. Fetched copy compared file-by-file (`knowledge/research/ext_traefik-modsecurity_reclaim_table/notes.md`).

## Decisions

### Upstream delta is thin; only one of the three files carries a real fix

Measured (`git diff --no-index` of the fetched upstream copy vs this tree):

| File | Delta |
|------|-------|
| `default.go` | byte-identical |
| `table.go` | 10 lines, local-variable renames only (`v` → `stored` / `created`) |
| `table_test.go` | +6 lines: a `waitUntil` in `TestTable_HashChangeProof` before reading `ended` |

Decision: port both. The renames are a readability win with zero behavior change (`skill:sbs-dev-commandments`), and the test wait is the upstream acknowledgement of the Close/dispose race. The ticket's premise — "this component was fixed and improved upstream" — is only partly true: upstream fixed the *symptom* in the test. This change fixes the *cause* in the component (below) and keeps the assertion strict.

### Root cause 1 (reproduced): Close runs on another goroutine, so the dispose log is not a completion signal

`TestTable_HashChangeProof` reads `ended` immediately after `waitKeyMsg(MsgDispose, "A")` and fails with `ended: []`.

Reproduction (this worktree, 8 background CPU burners on an 8-thread box, `go test ./pkg/reclaim/ -count=8`):

```
--- FAIL: TestTable_HashChangeProof (0.02s)
    table_test.go:372: ended: []
```

Decision: fix the component, not the test — `reclaim_dispose` must be a real completion signal ("Close has returned"). Implemented additively, after the human ruled that this package is a copy shared across projects and nothing may be removed (see the resolved question below): the lifetime context, its cancel, and the `waitCtx(life)` goroutine all stay, the slot gains a `valueClosed` channel that the goroutine closes after `stopValue`, and `fire` and `Reset` wait on it before logging dispose. Cost: a slow `Close()` now blocks the timer goroutine (or `Reset`) instead of a detached one — acceptable for this table (one entry per database, `Close` stops a ticker), and it is the same exposure the lost-create path already has at `pkg/reclaim/table.go:140`.

An earlier draft of this decision said `fire` and `Reset` would call `stopValue` inline and the goroutine would go away. That was reversed; it is kept in `design.md` as the rejected alternative.

Consequence: upstream's `waitUntil` becomes redundant here. Keep the strict immediate read so a regression fails, and add a dedicated invariant test.

### Root cause 2 (reproduced): a test demands that reclaim win a 3 ms wall-clock race

`TestTable_ReclaimRacesFire` (`pkg/reclaim/table_test.go:769`) uses `grace = 3ms`, waits for `reclaim_orphan`, then requires the following `Open` to return the *same* pointer. If the `AfterFunc` fires first — legal, correct behavior — the test fails with `round N reclaim lost the value`.

Reproduction: 10 failures in 25 runs under the same CPU load. This is the dominant flake, and CI is worse than the repro box: `go test -v ./...` on `ubuntu-latest` runs packages concurrently on a 2-4 vCPU runner (`.github/workflows/ci.yml:38`).

Decision: keep the stress loop but make its assertions outcome-tolerant, the way the sibling `TestTable_ZeroGraceOpenRacesCancel` (`table_test.go:811`) already does — same pointer means the incarnation must stay alive while held; different pointer means fire won and the old value must have been closed. The invariant "a late `fire` must not dispose a live incarnation" stays covered deterministically by `TestTable_StaleFireAfterReclaimNoops`, which calls `tab.fire` directly with the stale generation.

Same defect class, same fix, in tests that assert the reclaim branch on a timer window: `TestTable_OpenDuringGraceReclaims` (80 ms) gets a grace long enough that the branch is certain rather than probable.

### Root cause 3 (analysis, not reproduced): `reclaim_orphan` can be logged after `reclaim_dispose`

`drop` arms the grace timer, unlocks, and *then* logs `reclaim_orphan` (`pkg/reclaim/table.go:211-215`). With a small grace the `AfterFunc` can win and log `reclaim_dispose` first, so the per-key sequence assertions (`[put, bind, orphan, dispose]`) are unsound at small grace, and an operator reading debug logs can see dispose before orphan.

The spec forbids the obvious fix: `openspec/specs/std_go_reclaim_context-lease/spec.md` — "Log lines MUST NOT be emitted while the table mutex is held."

Decision: order it without logging under the mutex. `drop` marks the slot as *arming* under the lock, unlocks, logs `reclaim_orphan`, then re-locks and arms the timer only if the slot is still mapped and still has no holders. `bindLocked` treats `arming` exactly like an armed timer (reclaim), so a bind that lands in the window keeps the slot live. Grace therefore starts after the orphan line, and `reclaim_dispose` can only follow it.

Two corrections came out of code review. First, the re-lock must not bail on the generation it logged for: an `Open` can reclaim during the orphan line and release again, and that holder's own `drop` returns early on `arming`, so bailing strands the slot mapped with no holders and no timer, and its value is never closed. It arms against the generation it reads at re-lock instead. Second, the arming-window bind is *not* ordered against the orphan line — the bind takes the mutex that `drop` released in order to log — so neither the spec nor a test claims that order.

### Test coverage the ticket asks for

Untested surface on `origin/master`, to be added: the `waitCtx` nil-`Done` polling branch (`table.go:76-83`, only reachable with a context whose `Done()` is nil), `stopValue` on a value without `Close()`, `ResetWith` grace actually applied to the new process table, `Default()` under concurrent first use, repeated orphan → reclaim → orphan → dispose cycles on one key, holder-map cleanup across many opens, the logger-ownership rule (orphan/dispose use the *last* `Open`'s logger) which the spec states but no test checks, key independence at scale, concurrent `Reset` against in-flight `Open`, and a goroutine-count check that proves no goroutine outlives the key.

Also raising `waitBudget` (`table_test.go:17`) from 2 s: it is a timeout guard, not an assertion, and 2 s of 1 ms polls is thin on a loaded runner.

### Integration tests in `pkg/dbwrappers` are the same defect class

`pkg/dbwrappers/reclaim_test.go` sets a 25 ms lease (`useShortLeases`) and then requires the reclaim `Open` to land inside it (`TestOpenBIN_SameHashReclaimKeepsTicker:118`, `TestOpenMMDB_SameHashReclaimKeepsTicker:238`), and uses `time.Sleep(80 * time.Millisecond)` where the dispose tests need a dispose to have happened. Not reproduced in 6 stressed runs, but it is the same race shape. Decision: harden in this change — generous lease for the tests that assert the reclaim branch, and poll for `reclaim_dispose` instead of sleeping a fixed 80 ms in the tests that assert dispose.

### Not taken

No change to `Open`'s public signature, no change to the message constants, no change to `DefaultGrace`, and no change to how `pkg/dbwrappers` keys or opens databases. The holder `watch` goroutine for a `context.Background()` holder still parks forever by design (no `Done` to close); that is a documented Yaegi accommodation, not this ticket.

## Open questions

- Q: Should `Close()` be called synchronously inside `fire` / `Reset` (blocking that goroutine) instead of on a per-slot goroutine?
  Decision: resolved — no. The human ruled that `pkg/reclaim` is a copy shared with `traefik-modsecurity` and edits must be additive, so the lifetime context and its goroutine stay. Instead the slot gets a `valueClosed` channel that the lifetime goroutine closes after `Close()`, and `fire` / `Reset` wait on it before logging dispose. Same determinism, nothing removed.
  By: implement

- Q: The spec forbids logging while `t.mu` is held, so how is `reclaim_orphan` ordered before `reclaim_dispose`?
  Decision: resolved — mark the slot `arming` under the lock, log outside it, then re-lock and arm only when the slot is still mapped and `graceGen` is unchanged. No log under the mutex, deterministic order.
  By: explore

- Q: Which CI runs actually failed on `pkg/reclaim`, and with which message?
  Decision: assumed — GitHub Actions job logs are not reachable from here (no `gh` CLI on this box, no Actions tool in the available MCP namespaces, raw log URLs return 403). Do not claim a specific CI run as evidence. Local reproduction under CPU contention is the evidence of record: `TestTable_HashChangeProof` and `TestTable_ReclaimRacesFire` both fail, and both are timing-dependent by construction.
  By: explore

- Q: Should `pkg/dbwrappers/reclaim_test.go` be hardened in this change even though it did not flake in the stress runs?
  Decision: assumed — yes, harden it. It asserts the same "reclaim wins a short timer window" shape that reproduced in `pkg/reclaim`, the edit is confined to test files, and leaving a known-fragile window in place would just move the CI flake.
  By: explore

- Q: Should the upstream `traefik-modsecurity` copy be updated with the component fix so the two trees do not drift further?
  Decision: resolved — yes, and it is a standing rule, not a one-off: the human confirmed this component is synced in both directions across projects. Every change here ports there and vice versa, which is why this change removes nothing. The port itself happens in that repo, so it stays a follow-up on this PR; the rule is now written in `knowledge/devdocs/std_go_reclaim.md`.
  By: implement

- Q: Does upstream have extra features this repo is missing (the human asked about "sleep and wake")?
  Decision: resolved — no. `pkg/reclaim` upstream on `main` is exactly three files (`default.go` 1086 B, `table.go` 6932 B, `table_test.go` 25600 B) and a code search for `Wake` or `Sleep` across that repo returns zero hits. The only upstream delta is variable renames in `table.go` plus a six-line `waitUntil` in `TestTable_HashChangeProof`. If such a feature exists it is in a different project, and the human has to name it.
  By: implement
