## Why

Dated BIN initialize copies the catalog file inside plugin `New`. A paid BIN can be gigabytes, so that copy holds Traefik’s whole router build. Hot-swap already exists; initialize does not use it. Operators also cannot see how large the opened BIN is.

## What Changes

- When a dated BIN exists and a bundled `defaultFile` is found, `OpenBIN` opens that seed first so `New` can return.
- The dated file is copied off the constructor path; when the copy is ready, hot-swap replaces the seed handle.
- If the bundled seed is missing, initialize keeps today’s sync dated copy.
- `BIN initialized` and `BIN hot-swapped` log `size_bytes` of the opened file.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_database_bin-copy`: seed-first initialize when a dated BIN and bundled seed both exist; `size_bytes` on init and hot-swap logs.

## Impact

- `pkg/dbwrappers/bin.go` (`initialize`, `createLocalCopy`, `hotSwap`)
- `pkg/dbwrappers/bin_test.go`
- Usage: `knowledge/devdocs/core_geoblock_database_wrapper.md`
