# Specs
change: reclaim-value-sleep-wake-lifecycle

FindSpecHost verdicts (one per delta):

- `new` std_go_reclaim_value-lifecycle — confidence high. The four events a stored value
  receives is a capability `context-lease` does not name: that leaf is about how a holder leases
  a value, this one is about what the value itself is told. Candidates considered:
  `std_go_reclaim_context-lease` (fold — rejected, past the small-adjustment threshold),
  `core_geoblock_database_wrapper-reclaim` (wrong family, product-side).
- `fold` std_go_reclaim_context-lease — confidence high. Grace redefined, create-once reworded
  for register-before-create, logging requirement extended, plus one added requirement for the
  close-before-dispose completion guarantee. Leaf name still names the unit.
- `fold` core_geoblock_database_wrapper-reclaim — confidence high. One added requirement: an
  unheld wrapper sleeps instead of staying fully live. The leaf already owns wrapper reclaim.
- `fold` core_geoblock_database_url-download — confidence medium. One added requirement: the
  update loop stops deterministically and can be restarted. The leaf already owns the download
  component; `core_geoblock_database_source-catalog` is about catalog rows, not the loop.

No allowlist change: `std`/`go` and `core`/`geoblock` are already in `openspec/specs/domains.md`,
and `reclaim` / `database` are already components on `openspec/specs/map.md`. Only a new leaf.

- added std_go_reclaim_value-lifecycle
- modified std_go_reclaim_context-lease
- modified core_geoblock_database_wrapper-reclaim
- modified core_geoblock_database_url-download
