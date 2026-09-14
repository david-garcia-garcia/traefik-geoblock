## 1. Replace pkg/reclaim with v1.0.1

- [ ] Copy `reclaim/table.go` from traefik-middleware-utilities v1.0.1 (`28da9ab0`) into `pkg/reclaim/table.go`
- [ ] Delete `pkg/reclaim/default.go` (`Default`, package `Open`, `Reset`, `ResetWith`, `NewTable`)
- [ ] Reshape `pkg/reclaim/table_test.go` onto `New(Config)` + `Hooks` (do not import utilities `yaegi_test.go`)

## 2. Wrappers table and Sleep/Wake

- [ ] Hold `var table = reclaim.New(reclaim.Config{Grace: reclaim.DefaultGrace})` in `pkg/dbwrappers`
- [ ] `OpenBIN` / `OpenMMDB`: `table.Open(..., Hooks{Sleep: updater.Stop, Wake: startUpdate, Close: close file})` closing over the pointer assigned inside `create`
- [ ] `Reset` / `ResetWith` operate on that table only (`ResetWith` = Reset then replace with `New(Config{Grace})`)

## 3. Plugin-root table

- [ ] Hold a plugin-root `*Table` in `plugin.go`; `bindPlugin` Opens on it with `Hooks{Close: Plugin.Close}`
- [ ] `ResetForTest` resets (and optionally replaces grace on) that table
- [ ] `plugin_instance_test.go` resets plugin-root and wrappers tables

## 4. Usage docs

- [ ] Update `knowledge/devdocs/std_go_reclaim.md` (and wrapper/plugin packets if usage is now wrong)
