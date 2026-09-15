---
url: https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/28da9ab0c4c1ec8fdfc98366bbd18dcfcb160d8b/knowledge/devdocs/std_go_reclaim.md
title: Reclaim usage packet in traefik-middleware-utilities
fetched: 2026-09-14
authority: source
ref: github.com/david-garcia-garcia/traefik-middleware-utilities@28da9ab0c4c1ec8fdfc98366bbd18dcfcb160d8b:knowledge/devdocs/std_go_reclaim.md
---

Caller creates the table with New(Config) and holds it. Avoid a process-wide table in reclaim. Avoid otherpkg.Table[*T].

Open: create takes no args. Required *slog.Logger. Hooks stored at put. Later Open does not run create, does not replace hooks, wakes if asleep. (value, nil) = live bind; canceled ctx at bind returns (nil, ctx.Err()). Avoid type-switch discovery on stored any.

Lifecycle: create -> (sleep -> wake)* -> sleep -> close. Close always after Sleep. EnforceCloseBeforeOpen defaults off. Set it for exclusive resources; cost is delaying next create (Traefik reload).

Grace is how long a sleeping value is kept. Zero = no wake window. Negative → DefaultGrace 10s.

Production: hold reclaim.New(Config{Grace: DefaultGrace}) at package scope; table.Open. Tests: New with short grace. Prefix keys.

Gotchas: AfterFunc for grace (Yaegi select miss). Later Open ignores its Hooks argument. Open blocks for Wake. Panicking create/Sleep/Wake unsticks the key. Table.Reset is tests only and unmaps first regardless of EnforceCloseBeforeOpen. Zero-value Table{} Open returns error. Construct with New(Config).
