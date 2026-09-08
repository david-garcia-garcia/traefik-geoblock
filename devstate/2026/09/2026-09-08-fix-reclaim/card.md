Developer review: in progress — 2026-09-08T15:52:00Z

## What this changes
**Operators.** A `reclaim_dispose` line in the logs now means the database handle behind that key is already closed, and it can no longer appear before the `reclaim_orphan` line for the same key. No configuration changes.

**Admin users.** None.

**Developers.** `pkg/reclaim` gains two guarantees and nothing is taken away. `reclaim_dispose` is emitted only after the stored value's `Close()` has returned, so it is a completion signal rather than a hint — the cost is that a value which blocks in `Close()` now blocks whoever ended the incarnation, which the spec forbids. `reclaim_orphan` is emitted before grace is armed, so the per-key sequence is stable at any grace including zero. The test file goes from 22 to 35 tests, covering the two new guarantees, the grace edge, and nine paths no test reached before. `pkg/reclaim` is a copy shared with `traefik-modsecurity`, so every edit here is additive by rule and ports back as a file copy.

**End users.** None.

## Motivation
`pkg/reclaim` is the table that lets one database handle survive a Traefik dynamic-config reload: the old middleware's context is canceled, and if the next `New` opens the same key inside the grace window, the same `*BIN`/`*MMDB` (and its update ticker) is reused instead of re-downloading and re-opening the file. Because that behavior is defined against wall-clock grace, the tests that prove it are written against wall-clock grace too — and on `master` two of them are unwinnable races rather than assertions.

On `master`, `go test -v ./...` fails intermittently in the `Test` job for two distinct reasons, both reproduced in this worktree under CPU contention:

1. `TestTable_HashChangeProof` waits for the `reclaim_dispose` log line and then reads the value's `Close()` side effect. But `fire` cancels the lifetime and logs dispose, while `Close()` runs on a *separate* per-slot goroutine watching that lifetime. The log wins the race and the test reports `ended: []`.
2. `TestTable_ReclaimRacesFire` sets grace to 3 ms, waits for `reclaim_orphan`, then requires the next `Open` to return the *same* pointer. When the 3 ms timer wins — which is correct component behavior — the test fails with `round N reclaim lost the value`. 10 of 25 stressed runs failed this way.

There is also a third, narrower defect: `drop` arms the grace timer and *then* logs `reclaim_orphan`, so with a small grace an operator can see `reclaim_dispose` before `reclaim_orphan` for the same key, and the per-key sequence assertions are unsound. Found by reading the code, then reproduced against `master`: the new `TestTable_OrphanPrecedesDisposeAtTinyGrace` recorded `[put, bind, dispose, orphan]` on round 57 of 300.

Not merging leaves every PR in this repo one coin flip away from a red `Test` job, which trains everyone to re-run CI instead of reading it — and the day a real reclaim regression lands, "just re-run it" is exactly the wrong reflex. It also leaves `reclaim_dispose` documented as the end-of-incarnation signal while it can be emitted before the database handle is actually closed.

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

## Merge readiness
Implement is complete: all 8 task groups are checked, the full local suite is green, and 25 stressed runs under eight CPU burners are green where `master` failed within two. Code review has not run yet; CI on the new head has not been measured. 2 items remain.

Priority: P2 — a coin-flip red `Test` job on every PR, plus a `reclaim_dispose` line that can precede both the orphan line and the actual `Close()`; the workaround today is re-running CI.
Reviewed head: 049a130
Owner decision: Not required for the code. One decision was taken by the human this round and is recorded below.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 5/6 | Fixed, proven against `master`, and stress-clean; code review and CI on this head are the remaining gates |
| CI proof | N/A | Not yet measured on 049a130 |
| Local tests proof | 6/6 | `go test ./...` green; `-count=25` on `pkg/reclaim` + `pkg/dbwrappers` green under eight CPU burners |
| Review resolution | 6/6 | No reviewer comments on PR #82 |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-08-fix-reclaim pushed | `git push` (049a130) |
| OpenSpec | reclaim-dispose-determinism valid | `openspec validate --strict reclaim-dispose-determinism` |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/82 | pr-host List/Create |
| CI | not measured on this head | pr-host check runs |
| Local tests | `go test ./...` all 10 packages ok; `-count=25` stressed green | shell, this worktree |
| Regression proof | 4 new tests fail on `origin/master`'s `table.go` in all 3 runs; a 5th fails within 57 rounds | scratch module `tmp-reclaim-proof2` |
| Lint | no findings; `golangci-lint` gofmt hits are the CRLF working tree and include untouched `default.go` | `golangci-lint run ./pkg/reclaim/... ./pkg/dbwrappers/...`, `gofmt -l` on LF copies is empty |
| PR comments | no comments | devstate/comments.md |

## Specs
| Spec | Change | Where |
| --- | --- | --- |
| `std_go_reclaim_context-lease` | 2 requirements added (dispose implies Close returned; either side of the grace edge is correct), 1 modified (orphan precedes dispose at every grace) | `openspec/changes/reclaim-dispose-determinism/specs/` |

## Follow-up issues
- Port this change to `david-garcia-garcia/traefik-modsecurity` `pkg/reclaim`. The two copies are kept in sync in both directions; the port is a file copy of `table.go` and `table_test.go` plus the `useLeases` change if that repo has the same integration tests. Not done here because it is a different repository.

## How this fits together
Local ticket `2026-09-08-fix-reclaim` runs in its own worktree on branch `2026-09-08-fix-reclaim`, which is already open as PR #82 against `master`; CI on that PR is the merge gate, and this card is the PR summary.

## Decision needed
| Question | Decision | By |
| --- | --- | --- |
| Which CI runs actually failed on `pkg/reclaim`, and with which message? | assumed — GitHub Actions job logs are not reachable from this box (no `gh` CLI, no Actions tool in the available MCP namespaces, raw log URLs 403). No specific CI run is claimed as evidence; local reproduction under CPU contention is the evidence of record. | explore |
| Should `pkg/dbwrappers/reclaim_test.go` be hardened too, even though it did not flake in the stress runs? | assumed — yes. It asserts the same "reclaim wins a short timer window" shape (25 ms lease) that reproduced in `pkg/reclaim`, and the edit stays inside test files. | explore |
| Should the upstream `traefik-modsecurity` copy get the same component fix so the two trees stop drifting? | resolved by the human — yes, and as a standing rule: this component is synced in both directions across projects, so changes here port there and vice versa, and nothing may be removed. Written into `knowledge/devdocs/std_go_reclaim.md`. | implement |
| Does upstream carry features this repo lacks (the human asked about "sleep and wake")? | resolved — no. Upstream `pkg/reclaim` is three files and a code search for `Wake`/`Sleep` in that repo returns zero hits. If such a feature exists it is in another project. | implement |
| Should the lifetime context and its per-slot goroutine be deleted, since closing inline is simpler? | resolved by the human — no. The no-removal rule applies even to code unreachable from this repo, so `fire`/`Reset` wait on a `closed` channel instead. | implement |

## Before merge
- [x] [P2] Make `reclaim_dispose` imply `Close()` has returned — done additively, via a `closed` channel the lifetime goroutine closes and `fire`/`Reset` wait on
- [x] [P2] Order `reclaim_orphan` before the grace timer is armed without logging under the table mutex
- [x] [P2] Make the race-stress tests assert both legal outcomes instead of requiring reclaim to win
- [x] [P3] Port the upstream variable renames in `table.go` for parity with `traefik-modsecurity`
- [x] [P3] Add the missing coverage (nil-`Done` holder, `ResetWith` grace, logger ownership, goroutine leak guard, repeated reclaim cycles)
- [x] [P3] Harden the 25 ms lease windows and fixed 80 ms sleeps in `pkg/dbwrappers/reclaim_test.go`
- [ ] [P2] Code review of the component change
- [ ] [P2] Green CI on PR #82

## Findings
| Finding | Where | Why it matters |
| --- | --- | --- |
| Making `Close()` precede the dispose line inverts the two observables, which broke three tests that had used a `Close`-side flag as a proxy for "dispose was logged" | `TestTable_OpenCancelDispose`, `TestTable_ZeroGraceEndsImmediately`, `TestTable_ConcurrentCancelLastHolders` | Surfaced only under the stress harness, one iteration after the fix looked done. All three now wait for the line and assert the flag after it, which makes each a second check of the new guarantee. The rule is in the devdocs gotchas. |

## Axis review
Not run yet — code review is the next phase.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | `std_go_reclaim_context-lease` | Same list as ## Specs; do not paste diff --stat |
| Tests in `pkg/reclaim` | 22 → 35 | The component this PR is about had no coverage of its own end-of-incarnation ordering |
| New tests that fail on `master` | 5 | A test that passes on the buggy code proves nothing |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 049a1306e0fd49d6154fd2a1b0f793f4a37c0f0d | Card must match the branch you measured |

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

### Rank-up moves
- Consider whether the holder `watch` goroutine should park forever for a `context.Background()` holder, or refuse that case outside tests.
