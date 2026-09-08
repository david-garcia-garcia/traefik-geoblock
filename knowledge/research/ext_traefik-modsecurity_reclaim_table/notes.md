# Reclaim table upstream delta

Sibling repo [traefik-modsecurity@645f4a2](https://github.com/david-garcia-garcia/traefik-modsecurity/tree/645f4a25d5023fc42e615e02240cab29799b39c5/pkg/reclaim) `pkg/reclaim` compared to this product at `origin/master` (`d10ad52`). Same author; shared context-lease table for Traefik plugin reload.

## default.go — identical

`default.go` on upstream `main` matches this product byte-for-byte: `Default`, package `Open`, `Reset`, `ResetWith` with `DefaultGrace` (`pkg/reclaim/default.go`).

Extract: [.sources/default.go.md](.sources/default.go.md)

## table.go — naming only

Logic, constants, state machine, and public API are the same. Upstream renames locals (`created`/`stored`/`value` vs local `v`) and the `stopValue` parameter name. No behavioral diff found in `Open`, `bindLocked`, `drop`, `fire`, or `Reset`.

Extract: [.sources/table.go.md](.sources/table.go.md)

## table_test.go — HashChangeProof race wait

Same test function set (27 `Test*` functions). Material delta: `TestTable_HashChangeProof` in upstream waits for `namedEnd.Close` via `waitUntil` after `MsgDispose`, with comment that Close runs on the life goroutine and can race the dispose log. Local reads `ended` immediately after `waitKeyMsg(MsgDispose)` without that wait (`pkg/reclaim/table_test.go` lines 364–372 vs upstream lines 364–373). That matches intermittent CI failure pattern on timing-sensitive reclaim tests.

Other diffs are formatting/indentation only.

Extract: [.sources/table_test.go.md](.sources/table_test.go.md)

## CI on this product master

Public Actions API (no auth): runs `34139964808` (2026-09-07) and `33307746472` (2026-08-30) on `master` concluded `failure` with job `Test` failed; Lint and Integration Tests succeeded. Job logs returned 403 without token — reclaim-specific failure text not verified from logs.

## Implication for this ticket

Bring upstream `TestTable_HashChangeProof` wait pattern at minimum. Optional: adopt upstream local naming in `table.go` for parity (no behavior change). Ticket also asks for additional coverage beyond upstream; upstream already includes stress tests (`TestTable_ReclaimRacesFire`, `TestTable_ZeroGraceOpenRacesCancel`, stale-fire guards, concurrent paths).
