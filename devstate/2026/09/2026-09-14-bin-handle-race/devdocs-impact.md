# Devdocs impact
change: bin-rwmutex-published-handle

## Units
- Wrapper — subsystem — `pkg/dbwrappers` / `knowledge/devdocs/core_geoblock_database_wrapper.md`
- Database lookup — subsystem — `core_geoblock_database_lookup` (BIN LookupRecord RLock SHALL)
- Reclaim — pattern — `core_geoblock_database_wrapper-reclaim` / `knowledge/devdocs/std_go_reclaim.md`

## Findings
- [x] language-gap  Published handle — `core_geoblock_database_wrapper` had How-to/Gotcha, no Language term
- [x] stale-usage  Wrapper getters — `core_geoblock_database_wrapper` Gotcha omitted Path/Version/SourcePath read lock and SourcePath skip-compare
