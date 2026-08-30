## Why

A missing `fields` / `fieldsPreconfigured` on an enabled catalog row fails plugin `New`, but the error only restates the validation. Operators cannot tell from the Traefik `ERR` line that the middleware never starts. BIN reclaim keys are `bin:` plus a hash, so `reclaim_put` cannot be joined to the catalog map key already in hand.

## What Changes

- The missing-column-map `Prepare` error states the implication: the plugin does not start and this middleware is not applied.
- BIN (and MMDB) reclaim table keys include the catalog source key and the config hash: `bin:<catalogKey>:<hash>`.
- Validity is unchanged: empty both maps still fails; no default preset.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_database_source-catalog`: missing `fields` / `fieldsPreconfigured` error text MUST name that the plugin does not start and the middleware is not applied.
- `core_geoblock_database_wrapper-reclaim`: BIN and MMDB process-table keys MUST include the catalog map key and the config hash.

## Impact

- `pkg/geoblock/config.go` (`resolveSourceFields`)
- Tests that match the current error substring
- `pkg/dbwrappers/bin.go` / `mmdb.go` (`binKey` / `mmdbKey`)
- Wrapper reclaim tests that capture table keys
- Usage: wrapper packet key shape; catalog gotcha for the error sentence
