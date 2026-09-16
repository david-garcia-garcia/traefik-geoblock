Developer review: in progress — 2026-09-16T10:19:47Z

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
Apply renamed the thirty package tests. Local `go test ./...` passed. Remote CI on this head is still running. 1 item remains.

Priority: P3 — Spec, docs, tests, or internal clarity — no current user or operator harm
Reviewed head: 9deb328
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | Rename landed; CI still in progress |
| CI proof | 3/6 | Run 35084387273 in progress — https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/35084387273 |
| Local tests proof | N/A | Remote PR; localTests passed is recorded on handoff |
| Review resolution | 6/6 | No open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-16-zzz-test-prefix pushed | `git` / origin `9deb328` |
| OpenSpec | zzz-test-file-prefix | `openspec/changes/zzz-test-file-prefix/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/91 | pr-host List |
| CI | build 35084387273 in progress https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/35084387273 | Lint, Test, Integration Tests in_progress |
| Local tests | passed | `go test ./...` ok on eight packages; tools/dbdownload has no tests |
| PR comments | no comments | inventory empty |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-09-16-zzz-test-prefix is on branch 2026-09-16-zzz-test-prefix with stub PR 91 into master. Apply prefixed the thirty first-party `*_test.go` files.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Must this change rewrite `knowledge/devdocs/core_geoblock_test-harness.md` Key files and how-to-use stems to `zzz_*`? | bounded incidental | assumed — do not rewrite those packets in apply; honor Out of scope. `*_test.go` globs stay true. Stale explicit stems wait for devdocsimpact / a follow-up note. | explore |

## Before merge
- [x] Prefix first-party `*_test.go` files with `zzz_` and keep the `_test.go` suffix [P3]
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
| Reviewed head | 9deb3284b1fac12871a866887d4ab3b90a20c761 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution versus DestBranch: `git mv` only to `zzz_<stem>_test.go`; local `go test ./...` passed.

Do we have a high-confidence way to reproduce? Yes, DestBranch still has unprefixed `*_test.go`; this head has thirty `zzz_*_test.go` files.

Is this the best way to solve the issue? Yes — rename only those files and keep `_test.go` so `go test` still discovers them.

### Evidence
What I checked:
- Thirty first-party `*_test.go` renamed; vendor excluded; no `zzz_proof_*` added
- `go test ./...` passed (worktree `9deb328`)
- CI run 35084387273 in progress

### Rank-up moves
None.
