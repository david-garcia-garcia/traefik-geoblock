Developer review: ready for review — 2026-09-16T10:29:27Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** Thirty first-party Go package tests now use basename `zzz_<oldstem>_test.go` (example: `pkg/dbwrappers/zzz_bin_test.go`, `zzz_plugin_instance_test.go`). Package names and `Test*` functions are unchanged.

**End users.** None.

## Motivation
This ticket is the filename convention for first-party Go package tests. On master those files use the unprefixed `*_test.go` stem and sit next to the production `.go` they cover, so they do not cluster in the explorer.

DestBranch still has that mix: `bin_test.go` next to `bin.go`, `plugin_instance_test.go` next to `plugin.go`, and the same pattern across `pkg/geoblock` and the other first-party packages. `go test` already keys on the `_test.go` suffix; the missing prefix is explorer noise, not a discovery gap.

If this PR does not land, later work keeps adding unprefixed test files and the explorer stays mixed. No operator, admin, or end-user path is wrong today.

## Merge readiness
Apply prefixed the thirty package tests. Local `go test ./...` passed. CI on this head succeeded. 0 items remain.

Priority: P3 — Spec, docs, tests, or internal clarity — no current user or operator harm
Reviewed head: 75190ca
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Rename landed; CI succeeded; no open comments |
| CI proof | 6/6 | Lint, Test, and Integration Tests succeeded — https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/35084958982 |
| Local tests proof | N/A | Remote PR; handoff localTests passed |
| Review resolution | 6/6 | No open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-16-zzz-test-prefix pushed | `git` / origin `75190ca` |
| OpenSpec | zzz-test-file-prefix archived | `openspec/changes/archive/2026-09-16-zzz-test-file-prefix/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/91 | pr-host List |
| CI | build 35084958982 succeeded https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/35084958982 | Lint success, Test success, Integration Tests success |
| Local tests | passed | `go test ./...` ok on eight packages |
| PR comments | no comments | inventory empty |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
- [ ] [Update test-harness Key files to `zzz_*_test.go`](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-16-zzz-test-prefix/knowledge/debt/2026-09-16-zzz-test-harness-stems.md) — Key files still cite unprefixed stems after the `zzz_` rename.

## How this fits together
Ticket 2026-09-16-zzz-test-prefix is on branch 2026-09-16-zzz-test-prefix with PR 91 into master. CI run 35084958982 succeeded.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Must this change rewrite `knowledge/devdocs/core_geoblock_test-harness.md` Key files and how-to-use stems to `zzz_*`? | bounded incidental | assumed — do not rewrite those packets in apply; honor Out of scope. `*_test.go` globs stay true. Stale explicit stems wait for devdocsimpact / a follow-up note. | explore |

## Before merge
None.

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-16-zzz-test-prefix/devstate/2026/09/2026-09-16-zzz-test-prefix/codereview_standards.md) — 0 total, 0 pending, 0 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-16-zzz-test-prefix/devstate/2026/09/2026-09-16-zzz-test-prefix/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-16-zzz-test-prefix/devstate/2026/09/2026-09-16-zzz-test-prefix/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-16-zzz-test-prefix/devstate/2026/09/2026-09-16-zzz-test-prefix/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-16-zzz-test-prefix/devstate/2026/09/2026-09-16-zzz-test-prefix/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-16-zzz-test-prefix/devstate/2026/09/2026-09-16-zzz-test-prefix/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-16-zzz-test-prefix/devstate/2026/09/2026-09-16-zzz-test-prefix/codereview_coverage.md) — 0 total, 0 pending, 0 completed

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs; do not paste diff --stat |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 75190ca56497a86f5ef53ba45c7583dc90c0c9d0 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution versus DestBranch: `git mv` only to `zzz_<stem>_test.go`; local `go test ./...` passed; CI succeeded.

Do we have a high-confidence way to reproduce? Yes, DestBranch still has unprefixed `*_test.go`; this head has thirty `zzz_*_test.go` files.

Is this the best way to solve the issue? Yes — rename only those files and keep `_test.go` so `go test` still discovers them.

### Evidence
What I checked:
- Thirty first-party `*_test.go` renamed; vendor excluded; no `zzz_proof_*` added
- `go test ./...` passed
- CI run 35084958982: Lint, Test, Integration Tests success
- Seven-axis review: all `none.`
- Usage Key files left stale on purpose (Out of scope); debt note written

### Rank-up moves
None.
