## 1. Reclaim Open logger

- [ ] 1.1 Add `logger *slog.Logger` to `Table.Open` and package `Open`. Nil → table logger → `slog.Default()`. Store last Open logger on the slot.
- [ ] 1.2 Log `reclaim_put` and `reclaim_dispose` at debug. Put/bind/reclaim use the causing Open’s logger; orphan/dispose use the slot logger. `Reset` dispose uses slot logger or table logger.
- [ ] 1.3 Update `pkg/reclaim` tests for the new signature and debug levels. Add coverage: nil logger; info handler hides put/dispose; debug handler shows them.

## 2. Call sites

- [ ] 2.1 `plugin.go` `bindPlugin`: build the plugin logger (same as `pluginLogger`) and pass it to `Open`.
- [ ] 2.2 `OpenBIN` and `OpenMMDB`: pass the wrapper logger into `reclaim.Open`.
- [ ] 2.3 Update remaining `reclaim.Open` / `Table.Open` call sites in this module (tests included).

## 3. Usage doc

- [ ] 3.1 Update `knowledge/devdocs/std_go_reclaim.md`: `Open` takes a logger; all five messages at debug.
