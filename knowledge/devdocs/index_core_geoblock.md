# geoblock

## Plugin packages
priority: normal
local: core_geoblock_plugin_packages.md
description: How the Traefik Yaegi plugin keeps entrypoints at module root and helpers under pkg/.

## Request mode
priority: normal
local: core_geoblock_plugin_request-mode.md
description: How Config mode splits GeoIP lookup from country allow/block on one ServeHTTP.

## Plugin instance
priority: normal
local: core_geoblock_plugin_instance.md
description: How New reuses one Plugin per middleware name and config hash.

## Database lookup
priority: normal
local: core_geoblock_database_lookup.md
description: How the plugin opens and merges enabled catalog sources.

## Wrapper
priority: normal
local: core_geoblock_database_wrapper.md
description: How this plugin opens one BIN or MMDB file and hot-swaps it.

## Source
priority: normal
local: core_geoblock_database_source.md
description: How this plugin resolves a catalog file and keeps it current.

## Test harness
priority: normal
local: core_geoblock_test-harness.md
description: How package tests, throughput gates, and Pester integration cases are added.
