## Purpose

Binds each format-wrapper singleton (one open BIN or MMDB file and its keep-current loop) to the Traefik `New` context through the reclaim lease, so a same-hash reload reclaims the wrapper and an unreclaimed hash is disposed after grace.

## Requirements

### Requirement: Wrapper open receives the New context
Opening a BIN or MMDB wrapper SHALL take the context passed to plugin `New`. The plugin MUST pass that context through catalog source open into the wrapper open. The wrapper MUST open that hash on the process `reclaim` table (`any`, caller asserts `*BIN` / `*MMDB`) with a create that watches the incarnation lifetime and, when that lifetime is canceled, stops the keep-current loop and closes the open file. BIN and MMDB keys SHALL be prefixed so they do not collide. A BIN key SHALL be `bin:<catalogKey>:<hash>` and an MMDB key SHALL be `mmdb:<catalogKey>:<hash>`, where `catalogKey` is the `databaseSources` map key and `hash` is the wrapper-config hash.

#### Scenario: New context reaches the wrapper
- **WHEN** plugin `New` is invoked with a context
- **THEN** the format wrapper opened for that instance is stored and bound to that context on the format table

#### Scenario: Reclaim key includes catalog map key
- **WHEN** a BIN wrapper is opened for catalog key `asnlite`
- **THEN** the process-table key starts with `bin:asnlite:`
- **AND** the remainder is the wrapper-config hash

### Requirement: Same hash shares one wrapper and reclaims across reload
Two opens with the same wrapper configuration SHALL share one file and one keep-current loop. When the bound contexts are cancelled and a later open uses the same configuration with a new live context before grace ends, the wrapper MUST stay open and the keep-current loop MUST keep running.

#### Scenario: Same-hash New after generation cancel
- **WHEN** two plugin instances share one wrapper configuration
- **AND** Traefik cancels their `New` contexts and calls `New` again with the same configuration before grace ends
- **THEN** lookups on the shared wrapper still succeed
- **AND** a second keep-current loop is not started

### Requirement: Unreclaimed hash is disposed after grace
When no live `New` context remains for a wrapper configuration and grace elapses without a same-hash open, the wrapper SHALL stop its keep-current loop, close its file, and leave the singleton map. A later open with that configuration SHALL create a new wrapper. Closing a merged Lookup MUST still not end a wrapper that other instances (or a pending reclaim) still need.

#### Scenario: Config hash no longer used
- **WHEN** the last plugin instance using configuration hash H is gone (its `New` context is Done)
- **AND** no open with hash H occurs during grace
- **THEN** H’s keep-current loop is stopped
- **AND** H’s file handle is closed

#### Scenario: Lookup Close does not dispose
- **WHEN** one merged Lookup is closed and another still holds the same wrapper configuration with a live `New` context
- **THEN** lookups on the remaining instance still succeed

#### Scenario: Dynamic config hash change disposes the old wrapper
- **WHEN** a wrapper is opened with configuration H1 and a context
- **AND** that context is cancelled
- **AND** a wrapper is opened with different configuration H2 and a new context
- **AND** grace for H1 elapses with no open of H1
- **THEN** reclaim logs show create H1, orphan H1, create H2, and dispose H1 after grace
- **AND** H2 lookups succeed
- **AND** H1’s keep-current loop is stopped

### Requirement: An unheld wrapper sleeps instead of staying fully live
While a BIN or MMDB wrapper is stored but held by nobody, it SHALL NOT keep its update loop
running. When the last holder of a wrapper's context is Done, the wrapper SHALL stop its update
loop and SHALL NOT return until that loop has finished, so no download or file write can happen
after it has been put to sleep. When the wrapper is opened again before its grace ends, it SHALL
restart the update loop and SHALL be usable for lookups when `Open` returns. Sleeping and waking
SHALL NOT change the wrapper's identity: the same wrapper is returned.

#### Scenario: An orphaned wrapper stops its update loop
- **WHEN** every holder of a wrapper's context is Done and the wrapper has been put to sleep
- **THEN** the wrapper's update loop is no longer running
- **AND** no further download for that source is started

#### Scenario: A woken wrapper serves lookups and updates again
- **WHEN** a sleeping wrapper is opened again before its grace ends
- **THEN** the same wrapper is returned
- **AND** a lookup against it succeeds
- **AND** its update loop is running again

#### Scenario: Disposal does not stop the update loop a second time
- **WHEN** a sleeping wrapper's grace elapses and it is disposed
- **THEN** the wrapper releases its database handle
- **AND** it does not stop an update loop that sleeping already stopped
