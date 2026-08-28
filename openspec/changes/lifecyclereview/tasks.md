## 1. Reclaim package (stdlib)

- [ ] 1.1 Add `pkg/reclaim` with `Set`, `NewSet(grace, logger)`, `Put`, `Bind`. Log `reclaim_put` / `reclaim_bind` / `reclaim_orphan` / `reclaim_reclaim` / `reclaim_dispose` with `key`. Imports: standard library only.
- [ ] 1.2 Package tests: Put once + two Binds; cancel + Bind before grace (no dispose); grace without Bind (dispose once); second Put ignored; independent keys. Use a short grace in tests.
- [ ] 1.3 Test hash-change sequence from logs: Put+Bind A, cancel A, Put+Bind B, wait grace → `reclaim_put` A, `reclaim_orphan` A, `reclaim_put` B, `reclaim_dispose` A; B not disposed.

## 2. Wrappers bind New ctx

- [ ] 2.1 Pass `ctx` from plugin `New` through vendor `New` into `OpenBIN` / `OpenMMDB`.
- [ ] 2.2 On wrapper **create**, `Put` dispose = `close` + delete map entry. On every open, `Bind` the `New` ctx. Product grace 10s.
- [ ] 2.3 Wrapper tests: same-hash cancel then open again before grace (one ticker, Lookup works). Do not change BIN hot-swap 10s close.
- [ ] 2.4 Wrapper test: open H1, cancel ctx, open H2 (config change), wait grace. Assert reclaim logs (create H1, orphan H1, create H2, dispose H1); H2 Lookup works; H1 loop stopped.

## 3. Usage docs

- [ ] 3.1 Write `knowledge/devdocs/std_go_reclaim.md` (Language + how to copy the package). Point `index.md` / `index_std_go.md` / `domains.md` if not already there.
- [ ] 3.2 Update `knowledge/devdocs/core_geoblock_database_wrapper.md`: `Open*` takes `ctx`; dispose is reclaim, not `provider.Close`.
