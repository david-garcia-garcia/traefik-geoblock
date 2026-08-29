# geoblock

## Plugin packages
priority: normal
local: core_geoblock_plugin_packages.md
description: How the Traefik Yaegi plugin keeps entrypoints at module root and helpers under pkg/.

## Plugin instance
priority: normal
local: core_geoblock_plugin_instance.md
description: How New reuses one Plugin per middleware name and config hash.

## Database provider
priority: normal
local: core_geoblock_database_provider.md
description: How the plugin constructs and calls the geo DatabaseProvider.

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
