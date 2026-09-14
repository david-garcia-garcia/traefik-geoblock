## prepare (2026-09-14)
phase: prepare
findings: none
fixed: none
skipped: none

## explore (2026-09-14)
phase: explore
findings: reproduced F-4 resurrection on BIN and MMDB after Close during first GET
fixed: none
skipped: HTTPGet context cancel; F-1 LookupRecord mutex (noted as debt)

## propose (2026-09-14)
phase: propose
findings: folded join/ignore-after-close into core_geoblock_database_wrapper-reclaim
fixed: none
skipped: none

## implement (2026-09-14)
phase: implement
findings: Stop joins; tick skips onUpdate after stop; BIN/MMDB ignore after close
fixed: updater.go join; bin.go atomic closed; mmdb.go closed flag; delayed_close_test.go; updater_test.go
skipped: HTTPGet context; F-1 LookupRecord mutex

## codereview (2026-09-14)
phase: codereview
findings: Test coverage 2 hard (hotSwap/open after Close unproven by delayed-download tests)
fixed: TestOpenBIN_HotSwapAfterClose; TestOpenMMDB_OpenAfterClose (facb6ac)
skipped: none

## devdocsimpact (2026-09-14)
phase: devdocsimpact
findings: 2 stale-usage (wrapper Sleep/Close join; source Stop join)
fixed: core_geoblock_database_wrapper.md; core_geoblock_database_source.md
skipped: none

## archive (2026-09-14)
phase: archive
findings: folded delta into core_geoblock_database_wrapper-reclaim
fixed: live spec sync; moved to openspec/changes/archive/2026-09-14-disposed-generation-stays-disposed
skipped: none
