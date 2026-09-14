# Specs
change: bin-rwmutex-published-handle
- fold core_geoblock_database_wrapper-reclaim
- fold core_geoblock_database_lookup

FindSpecHost:
- published-handle mutex: fold `core_geoblock_database_wrapper-reclaim` (high; candidates: wrapper-reclaim, lookup, bin-copy, std_go_reclaim_value-lifecycle, plugin_instance-reclaim)
- LookupRecord RLock + Get_all: fold `core_geoblock_database_lookup` (high; candidates: lookup, wrapper-reclaim)
