Developer review: in progress — 2026-09-08T21:03:38Z

## What this changes

**Operators.** None.

**Admin users.** None.

**Developers.** Nothing yet — the branch carries only the run bus for `2026-09-08-reclaim-lifecycle`; the four-event reclaim lifecycle and the table restructure are not written.

**End users.** None.

## Motivation

`pkg/reclaim` keeps one shared value per key — a GeoIP BIN or MMDB wrapper — alive while any Traefik middleware instance holds it, and for a grace window after the last holder goes away so that a config reload does not re-download the database. The value only ever gets two events: it is created, and it is closed.

On `master` that grace window is not cheap. When the last holder's context is Done the incarnation stays fully live: `pkg/dbsource.Updater` keeps its 24h ticker armed and the `ip2location` file handle stays open for a resource nobody is using. Worse, `Updater.Stop` closes its stop channel and returns without joining the goroutine, and `tick` never checks that channel before it downloads — so a wrapper that was already disposed can still write a database file into a directory a test's `TempDir` is deleting. Two first `Open` calls for one key also both run `create`, which for a GeoIP source is a duplicated download and a second file open, and one of the two results is thrown away.

If this does not merge, the idle window stays expensive, the disposed-updater write stays a live source of CI flakes in `pkg/dbwrappers`, and every new state added to the table has to be layered onto a per-slot `context.WithCancel` lifetime plus a parked goroutine that nothing outside the package can observe.

## Merge readiness

Prepare only: the branch, run bus, and review PR exist. 7 phases remain.

Priority: P2 — real operator-visible waste and a disposed-updater write that corrupts a directory during teardown, with a workaround (short grace) and limited blast radius.
Reviewed head: acc9394
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Branch pushed, CI not measured on this head |
| CI proof | 1/6 | Pushed and still not seen |
| Local tests proof | N/A | Before implement |
| Review resolution | 6/6 | No open PR comments on #83 |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-08-reclaim-lifecycle pushed | `git push -u origin` |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/83 | GitHub MCP create_pull_request |
| CI | not seen | GitHub MCP |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md absent |

## Specs
None.

## Follow-up issues
None.

## How this fits together
The local ticket spec is dumped at `devstate/2026/09/2026-09-08-reclaim-lifecycle/ticket/source.md`; branch `2026-09-08-reclaim-lifecycle` was cut from `origin/master` (`d10ad52`) into a dedicated worktree, and PR #83 targets `master`. CI runs on each push to that branch.

## Decision needed
None.

## Before merge
- [ ] [P2] Explore: settle the per-key owner goroutine question with evidence
- [ ] [P2] Propose and apply the four-event lifecycle plus the table restructure
- [ ] [P2] Fix `pkg/dbsource.Updater` stop/join and wire BIN + MMDB sleep/wake
- [ ] [P3] Seven-axis code review on Opus, devdocs impact, archive
- [x] Branch, run bus, and review PR exist

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | acc9394e7feb593ac49b509da9228146fa9df4e5 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: Not yet — no product delta versus `master`.

Do we have a high-confidence way to reproduce? Yes, the CPU-contention recipe (12 hidden background burners, `GOMAXPROCS=2`, `go test ./pkg/reclaim/ -count=15`) reproduces the package's flakes locally; `-race` is unavailable on this box (no C toolchain).

Is this the best way to solve the issue? Not yet decided — the owner-goroutine versus mutex question is explore's job.

### Evidence
What I checked:
- `pkg/reclaim/table.go` is 260 lines with a 7-field `slot` on `origin/master` (`d10ad52`), not the 320/9 the ticket quotes — that describes PR #82's branch.
- `pkg/dbsource/updater.go:106-121` `Stop` closes `u.stop` and returns; there is no `WaitGroup` and `tick` has no stop check (`origin/master`).
- `pkg/reclaim/table.go:223-238` `fire` logs `reclaim_dispose` right after `cancel()`, without joining the goroutine that runs `Close()` (`origin/master`).

### Rank-up moves
None.
