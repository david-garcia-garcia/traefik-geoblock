Developer review: in progress — 2026-09-08T14:16:53Z

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** Prepare only: bus folder, upstream reclaim research, requirement grounded in `pkg/reclaim` vs traefik-modsecurity `main`. No product code yet.
**End users.** None.

## Motivation
The context-lease table in `pkg/reclaim` keeps one shared incarnation per key while Traefik plugin holders come and go; when the last holder drops, a grace timer decides whether the value is disposed or reclaimed by a reload. On `master`, CI has intermittent `Test` job failures (runs `34139964808`, `33307746472`) while Lint and Integration stay green — the ticket points at reclaim timing tests. The sibling traefik-modsecurity repo holds the same package; research shows the material delta is an async wait fix in `TestTable_HashChangeProof`, not a production logic change. Without porting that fix and closing coverage gaps, flaky reclaim tests can block merges and erode confidence in database-wrapper lifecycle that depends on this table.

Priority: P3 — test and reliability clarity; no current production contract change on DestBranch.

## Merge readiness
Prepare complete. Explore is next. CI not measured on this branch yet.

Priority: P3 — test and reliability clarity; no current production contract change on DestBranch.
Reviewed head: 55d5265
Owner decision: Required. See Decision needed.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | N/A | Before implement; stub PR only. |
| CI proof | N/A | Branch pushed; checks not waited. |
| Review resolution | N/A | No PR comments inventoried. |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-08-fix-reclaim pushed | origin/2026-09-08-fix-reclaim 55d5265 |
| OpenSpec | none | handoff.yaml change: none |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/82 | GitHub #82 |
| CI | not seen | not waited |
| PR comments | none | empty inventory |
| Qualify | qualified-with-gaps | handoff.yaml |
| Research | ext_traefik-modsecurity_reclaim_table | knowledge/research/ |

## Specs
None.

## Follow-up issues
None.

## How this fits together
Local ticket → prepare bus + upstream reclaim research → stub PR #82 → explore next (coverage scope, CI log confirmation).

## Decision needed
| Question | Decision | By |
| --- | --- | --- |
| Did failed master CI runs fail on pkg/reclaim tests? | assumed — yes, given ticket and HashChangeProof race; logs not fetched (403) | prepare |
| Port naming-only table.go upstream diff? | assumed — optional; behavior identical | prepare |
| Net-new tests beyond upstream port? | assumed — explore will size gaps after port | prepare |

## Before merge
Explore → propose → implement reclaim test fix and any added coverage → codereview with OPUS on reclaim → CI green → delivery card update.

## Findings
None (prepare only).

## Agent review details

### Security
None.

### Performance
None.

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 | Prepare only |
| Open reviewer comments walked | 0 | New stub PR |
| Reviewed head | 55d5265 | Prepare commit on branch |

### Stored data model
None.

### Technical review
Best possible solution: not evaluated (prepare).
Do we have a high-confidence way to reproduce? Partial — upstream identifies HashChangeProof race; CI log proof pending.
Is this the best way to solve the issue? TBD at implement.

### Evidence
What I checked:
- Fetched traefik-modsecurity main pkg/reclaim (default.go, table.go, table_test.go)
- Compared to local pkg/reclaim
- GitHub Actions API: two recent master Test failures
- git log -- pkg/reclaim (3 commits)

### Rank-up moves
None.
