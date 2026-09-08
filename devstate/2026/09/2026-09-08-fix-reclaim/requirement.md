# Requirement
IssueKey: 2026-09-08-fix-reclaim

## Problem
Intermittent CI failures on `master` are attributed to `pkg/reclaim`. The sibling `traefik-modsecurity` repo is believed to hold fixes and stronger tests for the same package.

## Current (code)
- `pkg/reclaim/default.go` — process-wide `Default`/`Open`/`Reset`/`ResetWith`; matches upstream `default.go` (`knowledge/research/ext_traefik-modsecurity_reclaim_table/notes.md`).
- `pkg/reclaim/table.go` — grace-lease `Table` with `Open`, `bindLocked`, `drop`, `fire`, `Reset`; behavior matches upstream; local variable names differ only (`v` vs `created`/`stored`).
- `pkg/reclaim/table_test.go` — 27 tests including race stress (`TestTable_ReclaimRacesFire`, `TestTable_ZeroGraceOpenRacesCancel`). `TestTable_HashChangeProof` reads `ended` immediately after dispose log without `waitUntil` for async `Close` (upstream adds wait at lines 368–373).
- `pkg/dbwrappers/reclaim_test.go` — integration tests for BIN/MMDB reclaim via `reclaim.Open` (`TestOpenBIN_*`, `TestOpenMMDB_*`).
- `openspec/specs/std_go_reclaim_context-lease/spec.md` — spec for reclaim lifecycle and debug log constants.
- `knowledge/devdocs/std_go_reclaim.md` — usage doc for reclaim table.
- CI: GitHub Actions runs `34139964808` (2026-09-07) and `33307746472` (2026-08-30) on `master` failed job `Test`; Lint/Integration succeeded. Reclaim-specific log lines not verified (job logs 403 without auth).
- Git history: three commits on `pkg/reclaim` — `#61` lifecyclereview, `#65` lifetime context, `#73` Open logger (#8902fd8).

## Desired
- Port upstream `pkg/reclaim` delta from `traefik-modsecurity` `main` (at minimum the `TestTable_HashChangeProof` race wait; optional naming parity in `table.go`).
- Add or extend test coverage for this critical component beyond current tree.
- Code review phase uses OPUS model for this component (process note for codereview skill).
- Done when: PR submitted, CI green, delivery card updated (ticket acceptance).

## Affected
- `pkg/reclaim/` (primary)
- Possibly `pkg/dbwrappers/reclaim_test.go` if integration gaps found
- `openspec/specs/std_go_reclaim_context-lease/spec.md` if behavior or coverage requirements change
- `knowledge/devdocs/std_go_reclaim.md` if usage invariants shift

## Out of scope
- Implementing during prepare.
- Refactoring unrelated packages.
- Forcing `table.go` rename-only upstream diff unless implement chooses parity.
- Resolving unrelated open PR #75.

## Unknowns
- Whether failed CI runs (`34139964808`, `33307746472`) failed specifically on `pkg/reclaim` tests (logs not fetched).
- What additional test coverage beyond upstream is still missing after porting upstream delta.

## Tensions
- Ticket assumes upstream has production fixes; research shows `default.go` identical and `table.go` naming-only — main upstream delta is `TestTable_HashChangeProof` async wait.
- Local already has extensive stress tests; ticket still asks to “improve test coverage” — scope for net-new tests vs port-only needs explore.
- Ticket requests OPUS at codereview; not a product requirement for prepare qualify.
