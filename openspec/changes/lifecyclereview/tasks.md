## 1. Reclaim package (stdlib)

- [ ] 1.1 Add `pkg/reclaim` with `Set`, `NewSet(grace)`, `Put(key, dispose)`, `Bind(key, ctx)`. Imports: standard library only.
- [ ] 1.2 Package tests: Put once + two Binds; cancel + Bind before grace (no dispose); grace without Bind (dispose once); second Put ignored; independent keys. Use a short grace in tests.

## 2. Wrappers bind New ctx

- [ ] 2.1 Pass `ctx` from plugin `New` through vendor `New` into `OpenBIN` / `OpenMMDB`.
- [ ] 2.2 On wrapper **create**, `Put` dispose = `close` + delete map entry. On every open, `Bind` the `New` ctx. Product grace 10s.
- [ ] 2.3 Wrapper tests: same-hash cancel then open again before grace (one ticker, Lookup works); unreclaimed hash after grace (loop stopped, map empty). Do not change BIN hot-swap 10s close.

## 3. Usage docs

- [ ] 3.1 Write `knowledge/devdocs/std_go_reclaim.md` (Language + how to copy the package). Point `index.md` / `index_std_go.md` / `domains.md` if not already there.
- [ ] 3.2 Update `knowledge/devdocs/core_geoblock_database_wrapper.md`: `Open*` takes `ctx`; dispose is reclaim, not `provider.Close`.
