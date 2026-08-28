## 1. Reclaim table (stdlib file)

- [x] 1.1 Add non-generic `Table` in `pkg/reclaim` (`NewTable`, `Open`, `Default`). Stores `any`. Log `reclaim_put` and `reclaim_dispose` at info; `reclaim_bind`, `reclaim_orphan`, `reclaim_reclaim` at debug. Attr `key`. `table.go` imports: standard library only.
- [x] 1.2 Table tests: Open once + two holders; cancel + Open before grace (no dispose); grace without Open (dispose once); second create/dispose ignored; independent keys. Short grace in tests.
- [x] 1.3 Test hash-change sequence from logs: Open A, cancel A, Open B, wait grace → `reclaim_put` A, `reclaim_orphan` A, `reclaim_put` B, `reclaim_dispose` A; B not disposed.
- [x] 1.4 Process singleton: `Open` and `Default().Open` return the same incarnation.

## 2. Wrappers bind New ctx

- [x] 2.1 Pass `ctx` from plugin `New` through vendor `New` into `OpenBIN` / `OpenMMDB`.
- [x] 2.2 `Open*` is `reclaim.Open` (create + bind + assert). Product grace 10s. Prefixed keys on the process table.
- [x] 2.3 Wrapper tests: same-hash cancel then open again before grace (one ticker, Lookup works). Do not change BIN hot-swap 10s close.
- [x] 2.4 Wrapper test: open H1, cancel ctx, open H2 (config change), wait grace. Assert reclaim logs; H2 Lookup works; H1 loop stopped.
- [x] 2.5 Plugin `New` integration: ctx bind; same-hash reclaim after generation cancel; unreclaimed dispose; provider Close does not dispose; hash-change dispose. Short grace. BIN and one MMDB provider.

## 3. Usage docs

- [x] 3.1 Write `knowledge/devdocs/std_go_reclaim.md` (import `pkg/reclaim`, assert in the owner). Point `index.md` / `index_std_go.md` / `domains.md` if not already there.
- [x] 3.2 Update `knowledge/devdocs/core_geoblock_database_wrapper.md`: `Open*` takes `ctx`; dispose is the table, not `provider.Close`.
