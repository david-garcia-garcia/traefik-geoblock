Developer review: ready to merge — 2026-09-08T17:08:23Z

## What this changes
**Operators.** A `reclaim_dispose` line in the logs now means the database handle behind that key is already closed, and it can no longer appear before the `reclaim_orphan` line for the same key. No configuration changes.

**Admin users.** None.

**Developers.** `pkg/reclaim` gains three guarantees and nothing is taken away. `reclaim_dispose` is emitted only after the stored value's `Close()` has returned, so it is a completion signal rather than a hint — the cost is that a value which blocks in `Close()` now blocks whoever ended the incarnation, which the spec forbids. `reclaim_orphan` is emitted before grace is armed, so the per-key sequence is stable at any grace including zero. And grace now belongs to the incarnation rather than the table: `Open` takes a `grace`, the `Open` that creates the value fixes it, a later `Open` cannot move it, and `reclaim.TableGrace` means "take the table's" — which is what all three production call sites pass, so their behavior is unchanged. The package goes from 26 to 47 tests, covering the new guarantees, the grace edge, and the paths no test reached before. `pkg/reclaim` is a copy shared with `traefik-modsecurity`, so every edit here is additive by rule and ports back as a file copy.

**End users.** None.

## Motivation
`pkg/reclaim` is the table that lets one database handle survive a Traefik dynamic-config reload: the old middleware's context is canceled, and if the next `New` opens the same key inside the grace window, the same `*BIN`/`*MMDB` (and its update ticker) is reused instead of re-downloading and re-opening the file. Because that behavior is defined against wall-clock grace, the tests that prove it are written against wall-clock grace too — and on `master` two of them are unwinnable races rather than assertions.

On `master`, `go test -v ./...` fails intermittently in the `Test` job for two distinct reasons, both reproduced in this worktree under CPU contention:

1. `TestTable_HashChangeProof` waits for the `reclaim_dispose` log line and then reads the value's `Close()` side effect. But `fire` cancels the lifetime and logs dispose, while `Close()` runs on a *separate* per-slot goroutine watching that lifetime. The log wins the race and the test reports `ended: []`.
2. `TestTable_ReclaimRacesFire` sets grace to 3 ms, waits for `reclaim_orphan`, then requires the next `Open` to return the *same* pointer. When the 3 ms timer wins — which is correct component behavior — the test fails with `round N reclaim lost the value`. 10 of 25 stressed runs failed this way.

There is also a third, narrower defect: `drop` arms the grace timer and *then* logs `reclaim_orphan`, so with a small grace an operator can see `reclaim_dispose` before `reclaim_orphan` for the same key, and the per-key sequence assertions are unsound. Found by reading the code, then reproduced against `master`: the new `TestTable_OrphanPrecedesDisposeAtTinyGrace` recorded `[put, bind, dispose, orphan]` on round 57 of 300.

Not merging leaves every PR in this repo one coin flip away from a red `Test` job, which trains everyone to re-run CI instead of reading it — and the day a real reclaim regression lands, "just re-run it" is exactly the wrong reflex. It also leaves `reclaim_dispose` documented as the end-of-incarnation signal while it can be emitted before the database handle is actually closed.

The end of an incarnation on `master`, where the dispose line does not wait for anything:

```mermaid
sequenceDiagram
    participant T as grace timer
    participant F as fire()
    participant L as life goroutine
    participant Test as TestTable_HashChangeProof
    T->>F: grace elapsed
    F->>F: cancel(life)
    F->>L: (async) waitCtx wakes
    F->>Test: log reclaim_dispose
    Test->>Test: read ended -> [] FAIL
    L->>L: stopValue -> Close() (too late)
```

The same path on this branch. The goroutine stays; `fire` now waits for it:

```mermaid
sequenceDiagram
    participant T as grace timer
    participant F as fire()
    participant L as life goroutine
    participant Test as TestTable_HashChangeProof
    T->>F: grace elapsed
    F->>F: cancel(life)
    F->>L: waitCtx wakes
    L->>L: stopValue -> Close()
    L->>F: close(slot.valueClosed)
    F->>Test: log reclaim_dispose
    Test->>Test: read ended -> [1] PASS
```

## Merge readiness
Ready to merge. Implement, code review and archive are complete, and the per-slot grace the human asked for landed on top and was reviewed on its own. Across two rounds Opus returned 40 findings (13 hard); 35 are applied, 5 are argued and left with measurements. Three were real defects rather than style: an incarnation that could be stranded forever by the new arming window, two `pkg/dbwrappers` tests that were never taking the reclaim branch they claim to test, and a new grace test that could not fail for the behavior it was named after. The full local suite is green, stressed runs under twelve CPU burners at `GOMAXPROCS=2` are green where `master` failed within two, each mutation the second round found surviving the suite now dies, and all three CI jobs are green on the final head.

Priority: P2 — a coin-flip red `Test` job on every PR, plus a `reclaim_dispose` line that can precede both the orphan line and the actual `Close()`; the workaround today is re-running CI.
Reviewed head: ec6db3e
Owner decision: Not required for the code. Three decisions were taken by the human and are recorded below.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Fixed, reviewed twice on Opus, proven against `master` and against signature-preserving mutations, stress-clean, archived, and green on CI |
| CI proof | 6/6 | Run 34254850208 green on ec6db3e — Test, Lint, Integration Tests — https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/34254850208. The only commit after it is this card |
| Local tests proof | 6/6 | `go test ./...` green; `-count=15` on `pkg/reclaim` + `pkg/dbwrappers` green under twelve CPU burners at `GOMAXPROCS=2`, plus `-count=25` under eight burners before the review fixes |
| Review resolution | 6/6 | 40 axis findings across two rounds: 35 applied, 5 argued with measurements, 0 open; no reviewer comments on PR #82 |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-08-fix-reclaim pushed | `git push` (ec6db3e) |
| OpenSpec | change valid before archive; `std_go_reclaim_context-lease` valid after the fold; the one failing spec in the catalog (`core_geoblock_database_token-download-file`, no requirements) is pre-existing from PR #60 and untouched here | `openspec validate --strict reclaim-dispose-determinism`, `openspec validate --specs --strict` |
| Spec catalog | map refreshed, both librarian validators OK | `validate-spec-map.mjs --write`, `validate-spec-map.mjs`, `validate-artifact-names.mjs` |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/82 | pr-host List/Create |
| CI | run 34254850208 success on ec6db3e (Test, Lint, Integration Tests) https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/34254850208 | pr-host check runs |
| Local tests | `go test ./...` all 10 packages ok; `-count=15` stressed green at `GOMAXPROCS=2` under twelve burners | shell, this worktree |
| Code review | 2 rounds on Opus, 40 findings, 35 applied / 5 argued / 0 open | `devstate/2026/09/2026-09-08-fix-reclaim/codereview_*.md` |
| Regression proof | 4 new tests fail on `origin/master`'s `table.go` in all 3 runs; a 5th fails within 57 rounds; the 3 grace tests each kill a signature-preserving mutation | scratch modules `tmp-reclaim-proof2`, `%TEMP%\grace-mutate` |
| Lint | no findings; `golangci-lint` gofmt hits are the CRLF working tree and include untouched `default.go` | `golangci-lint run ./pkg/reclaim/... ./pkg/dbwrappers/...`, `gofmt -l` on LF copies is empty |
| PR comments | no comments | devstate/comments.md |

## Specs
| Spec | Change | Where |
| --- | --- | --- |
| `std_go_reclaim_context-lease` | 5 requirement blocks folded into the catalog: 2 added (dispose implies Close returned; either side of the grace edge is correct), 3 modified (orphan precedes dispose at every grace; `Open` takes a grace; grace is per incarnation) | [openspec/specs/std_go_reclaim_context-lease/spec.md](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-08-fix-reclaim/openspec/specs/std_go_reclaim_context-lease/spec.md), archived change at [openspec/changes/archive/2026-09-08-reclaim-dispose-determinism](https://github.com/david-garcia-garcia/traefik-geoblock/tree/2026-09-08-fix-reclaim/openspec/changes/archive/2026-09-08-reclaim-dispose-determinism) |

## Follow-up issues
- Port this change to `david-garcia-garcia/traefik-modsecurity` `pkg/reclaim`. The two copies are kept in sync in both directions; the port is a file copy of `table.go`, `default.go` and `table_test.go`, plus the equivalent test-support changes if that repo has the same integration tests. Not done here because it is a different repository.
- `pkg/dbsource`: `Updater.Stop` does not wait for the update goroutine, and `tick` never checks for stop before downloading, so a disposed wrapper can still write into a directory `t.TempDir` is removing. That is why `settleFirstTick` exists in `pkg/dbwrappers/reclaim_test.go`. The fix is to cancel the in-flight request and join the goroutine; out of scope for a reclaim ticket.

## How this fits together
Local ticket `2026-09-08-fix-reclaim` runs in its own worktree on branch `2026-09-08-fix-reclaim`, which is already open as PR #82 against `master`; CI on that PR is the merge gate, and this card is the PR summary.

## Decision needed
| Question | Decision | By |
| --- | --- | --- |
| Which CI runs actually failed on `pkg/reclaim`, and with which message? | assumed — GitHub Actions job logs are not reachable from this box (no `gh` CLI, no Actions tool in the available MCP namespaces, raw log URLs 403). No specific CI run is claimed as evidence; local reproduction under CPU contention is the evidence of record. | explore |
| Should `pkg/dbwrappers/reclaim_test.go` be hardened too, even though it did not flake in the stress runs? | assumed — yes. It asserts the same "reclaim wins a short timer window" shape (25 ms lease) that reproduced in `pkg/reclaim`, and the edit stays inside test files. | explore |
| Should the upstream `traefik-modsecurity` copy get the same component fix so the two trees stop drifting? | resolved by the human — yes, and as a standing rule: this component is synced in both directions across projects, so changes here port there and vice versa, and nothing may be removed. Written into `knowledge/devdocs/std_go_reclaim.md`. | implement |
| Does upstream carry features this repo lacks (the human asked about "sleep and wake")? | resolved — no. Upstream `pkg/reclaim` is three files and a code search for `Wake`/`Sleep` in that repo returns zero hits. If such a feature exists it is in another project. | implement |
| Should the lifetime context and its per-slot goroutine be deleted, since closing inline is simpler? | resolved by the human — no. The no-removal rule applies even to code unreachable from this repo, so `fire`/`Reset` wait on a `valueClosed` channel instead. | implement |
| Grace is a per-key property, not a table-wide one — where should it live, and who wins on a reclaim? | resolved by the human — on the slot, with the table's grace as the default: `Open` takes a `grace`, the creating `Open` fixes it, and a reclaiming `Open` cannot change it. `TableGrace` is the spelling for "take the table's" and is what all three production call sites pass. | graceslot |

## Before merge
- [x] [P2] Make `reclaim_dispose` imply `Close()` has returned — done additively, via a `valueClosed` channel the lifetime goroutine closes and `fire`/`Reset` wait on
- [x] [P2] Order `reclaim_orphan` before the grace timer is armed without logging under the table mutex
- [x] [P2] Make the race-stress tests assert both legal outcomes instead of requiring reclaim to win
- [x] [P3] Port the upstream variable renames in `table.go` for parity with `traefik-modsecurity`
- [x] [P3] Add the missing coverage (nil-`Done` holder, `ResetWith` grace, logger ownership, goroutine leak guard, repeated reclaim cycles)
- [x] [P3] Harden the 25 ms lease windows and fixed 80 ms sleeps in `pkg/dbwrappers/reclaim_test.go`
- [x] [P2] Code review of the component change — seven axes on Opus, 27 findings, 24 applied and 3 argued
- [x] [P2] Move grace onto the incarnation (`Open` takes a `grace`, `TableGrace` means the table's) at the human's request, and review that commit on Opus — 13 findings, 11 applied and 2 argued
- [x] [P2] Fold the delta into `std_go_reclaim_context-lease`, refresh the map, and archive the change
- [x] [P2] Green CI on PR #82 for the final head ec6db3e (run 34254850208: Test, Lint, Integration Tests)

## Findings
| Finding | Where | Why it matters |
| --- | --- | --- |
| Making `Close()` precede the dispose line inverts the two observables, which broke three tests that had used a `Close`-side flag as a proxy for "dispose was logged" | `TestTable_OpenCancelDispose`, `TestTable_ZeroGraceEndsImmediately`, `TestTable_ConcurrentCancelLastHolders` | Surfaced only under the stress harness, one iteration after the fix looked done. All three now wait for the line and assert the flag after it, which makes each a second check of the new guarantee. The rule is in the devdocs gotchas. |
| The arming window this change adds could strand an incarnation forever | `pkg/reclaim/table.go` `drop` | An `Open` that reclaimed and released inside the orphan window returned early on `arming`, and the original `drop` then bailed because the generation had moved — leaving the slot mapped with no holders, no timer, and a value never closed. `drop` now arms against the state it reads at re-lock. `TestTable_ReclaimAndReleaseInsideOrphanWindowStillArms` drives the interleaving with a log handler that blocks on the orphan line; it times out on the pre-fix code. Found by the performance axis, not by any test. |
| Two `pkg/dbwrappers` tests were never taking the reclaim branch they are named for | `TestOpenBIN_SameHashReclaimKeepsTicker`, `TestOpenMMDB_SameHashReclaimKeepsTicker` | Wiring up the event recorder the coverage axis asked for showed the recorded events were put, bind, bind — no orphan, no reclaim. Cancelling a holder and calling `Open` straight after usually lands before the holder's watcher goroutine has dropped, so it was a plain second bind and the pointer-equality assert was trivially true. Both now wait for `reclaim_orphan` first and assert the reclaim line; runtime fell to 0.05 s. The trap is in the devdocs gotchas. |
| The test named after per-slot grace could not fail for that behavior | `TestTable_GraceIsPerIncarnation` | Its table was built with the 5 s no-race grace while `waitBudget` is 10 s, so waiting for the zero-grace key's dispose was satisfied by the *table's* grace elapsing. Reverting `drop` to a table-wide grace still passed it in 5 s. The table is now `2*waitBudget`, which fails the revert at the dispose wait. Two further mutations also survived the whole suite and now die: `drop`'s inline zero-grace branch reading `t.grace` (which would instantly dispose a long-grace key on a zero-grace table), and the `reclaim.Open` façade discarding its `grace` argument, whose failure mode is silent. Found by mutation, not by reading. |

## Axis review
Round 1: all seven axes on Opus, each against the diff pinned at 995849a.

| Axis | Findings | Applied | Argued | Worst finding |
| --- | --- | --- | --- | --- |
| Standards | 5 (4 hard) | 5 | 0 | The slot's `closed` field named the slot, not the fact it carries — the exact conflation this change exists to separate. Now `valueClosed` |
| Nitpicks | 5 (3 hard) | 5 | 0 | Test names and locals that described the mechanism instead of the behavior (`armingGen`, `useLeases`, `...DisposesQuietly`) |
| Spec | 4 | 4 | 0 | The delta spec claimed `reclaim_orphan` always precedes `reclaim_dispose` "at every grace", which `Reset` does not honor. The requirement now states both exceptions |
| Security | 0 | — | — | No findings. The change adds no input handling, no logging of untrusted data, and no new lock ordering |
| Performance | 4 (1 hard) | 2 | 2 | **A real leak introduced by this change**: the new arming window could strand an incarnation mapped with no holders and no timer, its value never closed |
| Dead code | 2 | 1 | 1 | An unreachable nil-channel guard in `waitClosed`; the helper is gone and `fire`/`Reset` receive directly |
| Test coverage | 7 (2 hard) | 7 | 0 | The arming window — the state this change adds — had no test that entered it through `drop` |

The three argued findings are recorded in the axis files with the measurement that justifies them: two are wall-clock trims that would remove a regression guard (the 40 rounds first catch the defect on round 57, and the `pkg/dbwrappers` sleeps are covering a `pkg/dbsource` shutdown gap, now a follow-up), and one is a pre-existing test-only production symbol.

Round 2: the per-slot grace commit 5146d0c reviewed on Opus on its own, five axes.

| Axis | Findings | Applied | Argued | Worst finding |
| --- | --- | --- | --- | --- |
| Correctness and concurrency | 0 | — | — | No findings. `slot.grace` is written once in the composite literal under `t.mu` before the slot is published, and both reads are under the lock, so the field is effectively immutable after publication |
| Test coverage | 4 (1 hard) | 4 | 0 | **The headline test could not fail** — see ## Findings. Two more mutations survived: the inline zero-grace branch and the façade dropping its argument |
| Spec and artifact truth | 4 (2 hard) | 4 | 0 | `proposal.md` claimed callers were unaffected at the API level five lines after listing the three call sites that changed, and still counted three requirement blocks where there are now five |
| Naming and API shape | 4 | 3 | 1 | `TableGrace` is a `time.Duration`, so `NewTable(TableGrace)` compiles and silently means 10 s — the one reading its name denies. Documented as an `Open` argument only |
| Portability to the shared copy | 0 | — | — | No findings. Still stdlib-only, non-generic, `create` unchanged; the port stays a file copy plus one argument per call site |

The two argued findings are naming calls the reviewer already marked defensible: `TableGrace` is kept over `InheritGrace` because it names where the value comes from and the `NewTable` hazard is closed by its doc comment instead, and no `NoGrace = 0` constant is added because new API surface on a shared copy costs more than the devdocs line that says `0` is the aggressive value and `TableGrace` is the default.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | `std_go_reclaim_context-lease` | Same list as ## Specs; do not paste diff --stat |
| Tests in `pkg/reclaim` | 26 → 47 | The component this PR is about had no coverage of its own end-of-incarnation ordering (`go test -list`) |
| New tests that fail on `master` | 5 | A test that passes on the buggy code proves nothing. The tests added during review pin states `master` does not have (the arming flag, the `grace` parameter), so those are measured against a mutation of this branch instead |
| Mutations killed | 3 of 3 | The grace tests are measured the only way that means anything once `Open`'s signature changed: revert the behavior, keep the signature, confirm each test fails |
| Axis findings | 40 found / 35 applied / 5 argued / 0 open | Two rounds on Opus; three findings were defects in this change, not style |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | ec6db3eada429a47e4eff66d218311d9b44d9462 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: it fixes the component rather than adding waits to the tests, which is what upstream did, and it does so without removing anything — the shared-copy rule makes additive the only acceptable shape. The one alternative worth naming is deleting the lifetime goroutine and calling `Close()` inline; that is a smaller file and was rejected on the no-removal rule.

Do we have a high-confidence way to reproduce? Yes — eight background CPU burners plus `go test ./pkg/reclaim/ -count=25` fails `TestTable_ReclaimRacesFire` about 40% of runs and `TestTable_HashChangeProof` intermittently; both pass on an idle box.

Is this the best way to solve the issue? Yes — the upstream delta alone (a `waitUntil` in one test) hides cause 1 and does nothing for cause 2, which is the dominant failure.

### Evidence
What I checked:
- Upstream `pkg/reclaim` fetched and diffed file-by-file: `default.go` identical, `table.go` renames only, `table_test.go` +6 lines (`knowledge/research/ext_traefik-modsecurity_reclaim_table/notes.md`)
- `TestTable_HashChangeProof` fails `ended: []` under contention (`go test ./pkg/reclaim/ -count=8`)
- `TestTable_ReclaimRacesFire` fails `reclaim lost the value` in 10 of 25 stressed runs (`go test ./pkg/reclaim/ -count=25`)
- `pkg/dbwrappers` reclaim tests passed 6 stressed runs — latent, not reproduced (`go test ./pkg/dbwrappers/ -run "Reclaim|HashChange|OpenBIN|OpenMMDB" -count=6`)
- CI runs `go test -v ./...` on `ubuntu-latest` with Go 1.21 and no `-race` (`.github/workflows/ci.yml:38`)
- The mutex-logging constraint that rules out the naive ordering fix (`openspec/specs/std_go_reclaim_context-lease/spec.md`)

What I checked after the fix:
- `go test ./...` — all 10 packages ok
- `go test ./pkg/reclaim/ ./pkg/dbwrappers/ -count=25` under eight CPU burners — green (44.6 s and 17.4 s); the same harness failed `master` within two iterations
- The new tests built against `origin/master`'s `table.go` in a scratch module fail there in all 3 runs: `TestTable_DisposeLogFollowsClose`, `TestTable_ResetDisposeLogFollowsClose`, `TestTable_RepeatedReclaimCyclesKeepOneIncarnation`, `TestTable_ReclaimRacesFire`; `TestTable_OrphanPrecedesDisposeAtTinyGrace` fails there on round 57 of 300 with `[put, bind, dispose, orphan]`
- `openspec validate --strict reclaim-dispose-determinism` — valid
- Upstream `pkg/reclaim` re-checked for the features the human asked about: three files, zero hits for `Wake` or `Sleep` in the whole repo

What I checked after the review fixes (head 4735753):
- `go build ./...`, `go vet ./...`, `go test ./...` — all 10 packages ok
- `go test ./pkg/reclaim/ ./pkg/dbwrappers/ -count=8` under twelve CPU burners at `GOMAXPROCS=2` — green
- `TestTable_OpenInsideOrphanWindowKeepsIncarnationLive` `-count=5` green; the arming window is now entered through `drop` rather than by hand-setting the flag
- `TestTable_GoroutinesReturnToBaseline` at `slack=2` (was 8): 25 solo runs and 8 full-suite runs green
- The two `SameHashReclaimKeepsTicker` tests now record `reclaim_reclaim`; before the fix their event stream was put, bind, bind
- `openspec validate --strict reclaim-dispose-determinism` — valid

What I checked after the per-slot grace review (head 4b41e46), in a scratch copy of the package with the signature preserved:
- `drop` reverted to a table-wide grace (`t.grace` in both branches) — fails `TestTable_GraceIsPerIncarnation` (10.00 s timeout at the dispose wait), `TestTable_KeyGraceSurvivesAZeroGraceTable`, `TestTable_ReclaimDoesNotChangeTheGrace` and `TestDefault_OpenForwardsTheGrace`. Before the fix to the table's grace in that first test, this same revert passed it
- only `drop`'s inline zero-grace branch reverted — fails `TestTable_KeyGraceSurvivesAZeroGraceTable`, the case added for it
- the `reclaim.Open` façade passing `TableGrace` instead of the caller's grace — fails `TestDefault_OpenForwardsTheGrace`, the case added for it
- the unmutated copy green after each experiment; `go build`, `go vet`, `go test ./...` and `-count=15` stressed all green in the worktree
- `golangci-lint`'s gofmt hits are the CRLF working tree (`core.autocrlf=true`): untouched `pkg/logging` flags identically, and `gofmt -d` on LF copies of all three changed files is empty

### Rank-up moves
- Consider whether the holder `watch` goroutine should park forever for a `context.Background()` holder, or refuse that case outside tests.
