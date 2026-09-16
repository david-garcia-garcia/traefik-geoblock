Developer review: in progress — 2026-09-16T10:06:38Z

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
Prepare grounded the ask and opened stub PR 91. Product renames are not on this head yet. 2 items remain.

Priority: P3 — Spec, docs, tests, or internal clarity — no current user or operator harm
Reviewed head: 871ee8d
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Stub PR exists; CI not seen; no product delta yet |
| CI proof | 1/6 | Pushed; CI not seen |
| Local tests proof | N/A | Remote PR; implement has not run |
| Review resolution | 6/6 | No open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-16-zzz-test-prefix pushed | `git` / origin |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/91 | pr-host List/Create |
| CI | not seen | pr-host CI |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | inventory empty |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-09-16-zzz-test-prefix is on branch 2026-09-16-zzz-test-prefix with stub PR 91 into master. CI has not been seen.

## Explore Decisions
None.

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
| Reviewed head | 871ee8d42b81ff8efca5784feee3b61e5de16c26 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: DestBranch has no `zzz_` prefix on first-party tests; this head has not renamed them yet.

Do we have a high-confidence way to reproduce? Yes, list first-party `*_test.go` on `origin/master` (thirty unprefixed files).

Is this the best way to solve the issue? Yes — rename only those files and keep `_test.go` so `go test` still discovers them.

### Evidence
What I checked:
- Dest tree `pkg/` and root `plugin_instance_test.go` on `origin/master` (`509e6c0`)
- Thirty first-party `*_test.go`; vendor has none
- CI workflow has no basename glob (`.github/workflows/ci.yml`)
- Stub PR 91 open; comment inventory empty

### Rank-up moves
None.
