## Context

See proposal.md — Why. Dest has thirty first-party `*_test.go` files and none already start with `zzz_`. CI is `go test -v ./...`. `go test` and Yaegi exclusion both key on the `_test.go` suffix. Usage-packet Key-files stems are Out of scope.

## Goals / Non-Goals

**Goals:**
- Every first-party package test file on dest uses basename `zzz_<oldstem>_test.go` after apply.
- `go test ./...` still discovers the same tests.

**Non-Goals:**
- Rewriting file contents (package, helpers, assertions).
- Pester, vendor, hunt `zzz_proof_*`.
- Usage or archive path rewrites.

## Decisions

- **`git mv` only.** New path is the same directory plus `zzz_` in front of the current basename. Alternative: copy + delete — rejected; loses rename history. Alternative: `t_` prefix — rejected; the ask is `zzz_`. Alternative: `zzz_proof_` — rejected; that name means uncommitted hunt files on earlier tickets.
- **Re-list at apply.** Glob first-party `*_test.go` excluding `vendor/` and already-prefixed `zzz_*_test.go`. Prefix whatever is present then. Alternative: hard-code the dump's thirty paths — rejected; explore already assumed dest may gain a file.
- **`skip_specs: true`.** No plugin or lookup SHALL changes. Alternative: new `std_go_test_zzz-prefix` leaf — rejected; would invent a requirement to pass validation.

## Risks / Trade-offs

- [Risk] A later production file named `update.go` sorts after `zzz_*` tests. → Mitigation: accepted cost of the commissioned prefix; do not pick a different prefix.
- [Risk] `zzz_bin_test.go` is read as a hunt file. → Mitigation: hunt names are `zzz_proof_*`; do not add that infix.
- [Risk] IDE file nesting `bin.go` → `bin_test.go` breaks. → Mitigation: accepted; clustering is the job.

## Migration Plan

Rename on the IssueKey branch. Rollback is revert the rename commit. No deploy or Config step.
