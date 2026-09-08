# Devdocs impact
change: reclaim-dispose-determinism

## Units
- Reclaim — subsystem — `pkg/reclaim` (`knowledge/devdocs/std_go_reclaim.md`)
- Wrapper lease teardown — pattern — `pkg/dbwrappers/reclaim_test.go` (`knowledge/devdocs/core_geoblock_test-harness.md`)

## Findings
- [x] language-gap  Reclaim — "incarnation" carries the whole packet (it appears in the Table, Open, Grace and Lifetime entries and in every log-ordering rule) but had no term of its own, so the one word a reader must get right to read the logs was defined nowhere. Added, with the pointer that a reclaim keeps the same incarnation and `reclaim_put` is what separates two of them.
- [x] stale-usage  Reclaim — the How-to said tests use `ResetWith` without the fact this change added: `Reset` / `ResetWith` now block until every stored value's `Close()` has returned. That is exactly what makes them usable as real teardown, which is how `pkg/dbwrappers` now stops its update tickers before `t.TempDir` is removed.
- none for Wrapper lease teardown — `core_geoblock_test-harness.md` points at the files (`plugin_instance_test.go`, the `/reclaima|b|c` Pester context) and names no helper this change renamed, so nothing in it went stale. The reclaim-specific test rules live in the Reclaim packet's Gotchas, which is the right fold.

The other three Gotchas this change produced were written during implement and code review rather than here:
- a `Close()`-side flag is not a signal that `reclaim_dispose` was logged
- cancel-then-`Open` is usually a second bind, not a reclaim; wait for `reclaim_orphan` first
- `reclaim_orphan` precedes the dispose that ends grace, with the two stated exceptions

No `missing-packet` and no `wrong-fold`: the change touches one subsystem and that subsystem already owns a packet.
