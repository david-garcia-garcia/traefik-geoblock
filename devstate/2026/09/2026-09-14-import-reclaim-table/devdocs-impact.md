# Devdocs impact

Units: reclaim table (`std_go_reclaim`), plugin instance, database wrapper.

- std_go_reclaim: stale-usage → produced (Hooks, caller-owned New, no Default)
- plugin instance: stale-usage → produced (plugin-root table, ResetForTest)
- database wrapper: stale-usage → produced (Sleep/Wake updater)

No remaining findings.
