## Why

`BIN initialized` logs the temp copy as `path` and the first middleware name as `plugin`. Operators cannot join that line to the dated catalog file or tell that `plugin` is only who created the shared BIN. The temp basename is `IP2LOCATION_<date>_<time>_<nsec>.BIN` with no catalog key.

## What Changes

- `BIN initialized` and `BIN hot-swapped` log `source_path` (catalog or seed file) and keep `path` / `new_path` as the opened file.
- The BIN wrapper logger uses `owner_plugin` (creating middleware), not `plugin`.
- Temp copy basename is `bin_<catalogKey>_<unixNano>.BIN`.

## Capabilities

### New Capabilities

- `core_geoblock_database_bin-copy`: BIN temp-copy name and init/hot-swap log attrs (`path` + `source_path`, `owner_plugin`).

### Modified Capabilities

None.

## Impact

- `pkg/dbwrappers/bin.go` (`initialize`, `createLocalCopy`, `hotSwap`)
- `pkg/logging/logging.go` (`NewOwner`) and BIN open in `pkg/geoblock/plugin.go`
- Wrapper tests that assert init logs or temp names
- Usage: `knowledge/devdocs/core_geoblock_database_wrapper.md`
