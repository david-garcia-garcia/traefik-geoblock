# Specs
change: reclaim-dispose-determinism
- modified std_go_reclaim_context-lease — FindSpecHost: fold, confidence high. Candidates: std_go_reclaim_context-lease, core_geoblock_database_wrapper-reclaim, core_geoblock_plugin_instance-reclaim. The delta is the reclaim table's own end-of-incarnation and log-ordering contract, which this leaf already owns; the two core_geoblock leaves own how the plugin and the database wrappers consume the table, and neither changes.
