## Purpose

Names the BIN temp copy so an operator can join `/tmp` to a catalog key, and logs both the opened file and the catalog/seed file it was copied from, plus which middleware created the shared wrapper.

## Requirements

### Requirement: Temp copy basename includes format, catalog key, and Unix nanosecond
When BIN initialize or hot-swap copies a catalog or seed file into the process temp directory, the copy basename SHALL be `bin_<catalogKey>_<unixNano>.BIN`. `catalogKey` SHALL be the `databaseSources` map key with runes outside `[A-Za-z0-9._-]` replaced by `_`. When that key is empty, the basename SHALL be `bin_<unixNano>.BIN`. The prefix `IP2LOCATION_` and a wall-clock date-time-nanosecond triple MUST NOT be used.

#### Scenario: Dated catalog copy uses catalog key
- **WHEN** a BIN is opened for catalog key `paid` from a dated catalog file
- **AND** the local copy succeeds
- **THEN** the opened file basename matches `bin_paid_<digits>.BIN`

#### Scenario: Unsafe catalog key is sanitized
- **WHEN** a BIN is opened for catalog key `paid/prod`
- **AND** the local copy succeeds
- **THEN** the opened file basename matches `bin_paid_prod_<digits>.BIN`

### Requirement: Init and hot-swap log source_path and the opened path
`BIN initialized` SHALL include `path` (the file the SDK opened) and `source_path` (the dated catalog or seed file that copy was taken from). When no copy is made, `source_path` MAY equal `path`. `BIN hot-swapped` SHALL include `new_path` (the new opened file) and `source_path` (the new catalog or seed file).

#### Scenario: Init after copy logs both paths
- **WHEN** a BIN is opened from a dated catalog file and the local copy succeeds
- **THEN** the `BIN initialized` line includes `source_path` equal to that dated file
- **AND** `path` is the temp copy

### Requirement: BIN wrapper logger uses owner_plugin
When the plugin opens a BIN wrapper, the wrapper logger SHALL include `owner_plugin` set to that middleware name and MUST NOT include `plugin`. `owner_plugin` is the middleware that created this singleton, not a claim that later binders are excluded.

#### Scenario: First middleware is owner_plugin
- **WHEN** plugin `New` for middleware `traefik-geoblock@kubernetescrd` opens a BIN
- **THEN** `BIN initialized` includes `owner_plugin=traefik-geoblock@kubernetescrd`
- **AND** that line does not include `plugin=`
