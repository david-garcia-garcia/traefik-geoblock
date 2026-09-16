Developer review: in progress — 2026-09-16T10:16:15Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** OpenSpec change `zzz-test-file-prefix` (`skip_specs`) plans `git mv` of first-party `*_test.go` to `zzz_<stem>_test.go`. The files are not renamed yet.

**End users.** None.

## Motivation
This ticket is the filename convention for first-party Go package tests. On master those files use the unprefixed `*_test.go` stem and sit next to the production `.go` they cover, so they do not cluster in the explorer.

DestBranch still has that mix: `bin_test.go` next to `bin.go`, `plugin_instance_test.go` next to `plugin.go`, and the same pattern across `pkg/geoblock` and the other first-party packages. `go test` already keys on the `_test.go` suffix; the missing prefix is explorer noise, not a discovery gap.

If this PR does not land, later work keeps adding unprefixed test files and the explorer stays mixed. No operator, admin, or end-user path is wrong today.

## Merge readiness
Propose landed the OpenSpec change with `skip_specs`. Product file renames are not on this head yet. 2 items remain.

Priority: P3 — Spec, docs, tests, or internal clarity — no current user or operator harm
Reviewed head: aa1637e
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | Propose done; rename not applied; CI in progress on this head |
| CI proof | 3/6 | Run 35084072476 in progress — https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/35084072476 |
| Local tests proof | N/A | Remote PR; implement has not run |
| Review resolution | 6/6 | No open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-16-zzz-test-prefix pushed | `git` / origin `aa1637e` |
| OpenSpec | zzz-test-file-prefix | `openspec/changes/zzz-test-file-prefix/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/91 | pr-host List |
| CI | build 35084072476 in progress https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/35084072476 | Lint, Test, Integration Tests in_progress |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | inventory empty |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-09-16-zzz-test-prefix is on branch 2026-09-16-zzz-test-prefix with stub PR 91 into master. Propose recorded `zzz-test-file-prefix` with `skip_specs`; apply has not started.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Must this change rewrite `knowledge/devdocs/core_geoblock_test-harness.md` Key files and how-to-use stems to `zzz_*`? | bounded incidental | assumed — do not rewrite those packets in apply; honor Out of scope. `*_test.go` globs stay true. Stale explicit stems wait for devdocsimpact / a follow-up note. | explore |
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
| Reviewed head | aa1637e947fd53a2989e5a33828c087a5880a59c | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution versus DestBranch: `git mv` only, keep `_test.go`, `skip_specs` because no plugin SHALL changes.

Do we have a high-confidence way to reproduce? Yes, list first-party `*_test.go` on `origin/master` (thirty unprefixed files).

Is this the best way to solve the issue? Yes — rename only those files and keep `_test.go` so `go test` still discovers them.

### Evidence
What I checked:
- Change `openspec/changes/zzz-test-file-prefix/` complete (proposal, design, tasks; specs skipped)
- CI run 35084072476 in progress on `aa1637e`
- Prior run 35083718292 Test failed on the explore-card head (not this head)

### Rank-up moves
None.
