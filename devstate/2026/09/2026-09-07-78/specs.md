# Specs
change: bin-country-short-empty

FindSpecHost:
- fold `core_geoblock_database_lookup` (high) — BIN `country_short` `-` is empty so Combined can fill. Candidates: `core_geoblock_database_lookup`, `core_geoblock_database_source-catalog`.
- fold `core_geoblock_plugin_request-mode` (high) — BIN-only miss is `XX`, not marked written. Candidates: `core_geoblock_plugin_request-mode`.
