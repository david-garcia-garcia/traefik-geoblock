# Devdocs impact
change: disposed-generation-stays-disposed

## Units
- Wrapper — subsystem — `pkg/dbwrappers` Sleep/Close, BIN `hotSwap`, MMDB `open`/`swapReader`
- Source — subsystem — `pkg/dbsource/updater.go` Updater `Stop` join and `tick` skip of `onUpdate`

## Findings
- [x] stale-usage  Wrapper — `core_geoblock_database_wrapper` How-to said Sleep/Close stop the ticker; no join, ignore-after-close, or late-copy wording
- [x] stale-usage  Source — `core_geoblock_database_source` How-to named Start only; Stop did not join and tick still implied `onUpdate` after GET
