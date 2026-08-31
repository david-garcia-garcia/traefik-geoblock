## MODIFIED Requirements

### Requirement: Init and hot-swap log source_path and the opened path
`BIN initialized` SHALL include `path` (the file the SDK opened) and `source_path` (the file that handle was taken from). When no copy is made, `source_path` MAY equal `path`. `BIN hot-swapped` SHALL include `new_path` (the new opened file) and `source_path` (the new catalog or seed file).

#### Scenario: Init without a bundled seed copies dated first
- **WHEN** a BIN is opened from a dated catalog file
- **AND** no bundled defaultFile exists
- **AND** the local copy succeeds
- **THEN** the `BIN initialized` line includes `source_path` equal to that dated file
- **AND** `path` is the temp copy

#### Scenario: Seed-first init then swap logs dated on hot-swap
- **WHEN** a BIN is opened from a dated catalog file
- **AND** a bundled defaultFile exists
- **THEN** `BIN initialized` has `path` and `source_path` equal to that seed
- **AND** after the dated copy, `BIN hot-swapped` has `source_path` equal to the dated file
- **AND** `new_path` is the temp copy

## ADDED Requirements

### Requirement: Dated BIN initialize opens bundled seed first
When initialize would copy a dated catalog BIN and a bundled `defaultFile` exists, OpenBIN SHALL open that seed and return before the dated copy finishes. Lookups SHALL use the seed until hot-swap. When the dated copy is ready, the wrapper SHALL hot-swap to that copy. When no bundled seed exists, initialize SHALL copy the dated file before return.

#### Scenario: Seed is live before dated copy
- **WHEN** a dated catalog BIN exists
- **AND** defaultFile names an existing bundled seed
- **THEN** OpenBIN returns with Path equal to that seed
- **AND** LookupRecord succeeds before the dated temp copy exists

#### Scenario: No seed keeps sync dated copy
- **WHEN** a dated catalog BIN exists
- **AND** defaultFile is empty or not found
- **THEN** OpenBIN returns with Path a temp copy of the dated file

### Requirement: Init and hot-swap log size_bytes
`BIN initialized` and `BIN hot-swapped` SHALL include `size_bytes` equal to the byte length of the opened file.

#### Scenario: Init logs opened file size
- **WHEN** a BIN is opened
- **THEN** `BIN initialized` includes `size_bytes` equal to that opened file’s size

#### Scenario: Hot-swap logs opened file size
- **WHEN** a BIN hot-swaps to a new file
- **THEN** `BIN hot-swapped` includes `size_bytes` equal to that new opened file’s size
