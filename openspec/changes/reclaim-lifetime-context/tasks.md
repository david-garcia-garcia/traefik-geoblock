## 1. Reclaim table

- [ ] 1.1 Change `Table.Open` and package `Open` to `create func(life context.Context) (any, error)`. Drop `dispose`. Slot stores `cancel`. Cancel in `fire`, `Reset`, and lost create race.
- [ ] 1.2 Update `pkg/reclaim` tests: create receives `life`; assert `life.Done()` instead of a dispose callback. Keep `reclaim_dispose` log assertions.

## 2. Call sites

- [ ] 2.1 `OpenBIN` / `OpenMMDB`: create watches `life` then `close`. No dispose func.
- [ ] 2.2 `plugin.go` `bindPlugin`: create ignores `life`. No empty dispose.
- [ ] 2.3 Wrapper and plugin lifecycle tests still compile and assert unreclaimed `reclaim_dispose` / closed file.

## 3. Usage

- [ ] 3.1 Update `knowledge/devdocs/std_go_reclaim.md` Open / dispose language to lifetime context.
- [ ] 3.2 Update wrapper and plugin-instance packets so they do not document a dispose callback.
