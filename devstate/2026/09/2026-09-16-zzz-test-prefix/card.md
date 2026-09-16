Developer review: in progress — 2026-09-16T10:10:57Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

**End users.** None.

## Motivation
This ticket is the filename convention for first-party Go package tests. On master those files use the unprefixed `*_test.go` stem and sit next to the production `.go` they cover, so they do not cluster in the explorer.

DestBranch still has that mix: `bin_test.go` next to `bin.go`, `plugin_instance_test.go` next to `plugin.go`, and the same pattern across `pkg/geoblock` and the other first-party packages. `go test` already keys on the `_test.go` suffix; the missing prefix is explorer noise, not a discovery gap.

If this PR does not land, later work keeps adding unprefixed test files and the explorer stays mixed. No operator, admin, or end-user path is wrong today.

## Merge readiness
Explore recorded assumed decisions. Product renames are not on this head yet. 2 items remain.

Priority: P3 — Spec, docs, tests, or internal clarity — no current user or operator harm
Reviewed head: b95a3b4
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | Explore done; product rename not applied; CI still in progress |
| CI proof | 3/6 | Lint and Test succeeded; Integration Tests in progress — https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/35083509999 |
| Local tests proof | N/A | Remote PR; implement has not run |
| Review resolution | 6/6 | No open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-16-zzz-test-prefix pushed | `git` / origin `b95a3b4` |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/91 | pr-host List |
| CI | build 35083509999 in progress https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/35083509999 | Lint success, Test success, Integration Tests in_progress |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | inventory empty |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-09-16-zzz-test-prefix is on branch 2026-09-16-zzz-test-prefix with stub PR 91 into master. Explore assumed the rename-only path; apply has not started.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Must this change rewrite `knowledge/devdocs/core_geoblock_test-harness.md` Key files and how-to-use stems to `zzz_*`? | bounded incidental | assumed — do not rewrite those packets in apply; honor Out of scope. `*_test.go` globs stay true. Stale explicit stems wait for devdocsimpact / a follow-up note. | explore |
| Must a new spec leaf mandate the `zzz_` basename, or does the rename alone satisfy Desired? | additive incidental | assumed — no new spec leaf unless propose FindSpecHost names a host that already owns test-file naming. Default is rename-only plus tasks that `git mv` the Affected list. | explore |
| What if dest gains another first-party `*_test.go` after the dump? | additive asked | assumed — implement re-lists first-party `*_test.go` (exclude `vendor/`) at apply and prefixes whatever is present then. | explore |

## Before merge
- [ ] Prefix first-party `*_test.go` files with `zzz_` and keep the `_test.go` suffix [P3]
- [ ] CI on PR 91

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs; do not paste diff --stat |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | b95a3b4e3e8b07ca1706ac6ae2aeddd30e117584 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: DestBranch has no `zzz_` prefix on first-party tests; this head has not renamed them yet.

Do we have a high-confidence way to reproduce? Yes, list first-party `*_test.go` on `origin/master` (thirty unprefixed files).

Is this the best way to solve the issue? Yes — rename only those files and keep `_test.go` so `go test` still discovers them.

### Evidence
What I checked:
- Worktree first-party `*_test.go` count is thirty; none already `zzz_*`
- Usage packet `knowledge/devdocs/core_geoblock_test-harness.md` still describes `*_test.go`
- CI run 35083509999: Lint success, Test success, Integration Tests in_progress
- Stub PR 91 open; comment inventory empty

### Rank-up moves
None.
