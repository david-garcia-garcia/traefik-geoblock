## MODIFIED Requirements

### Requirement: Wrapper open receives the New context
Opening a BIN or MMDB wrapper SHALL take the context passed to plugin `New`. The plugin MUST pass that context through the provider constructor into the wrapper open. The wrapper MUST open that hash on the process `reclaim` table (`any`, caller asserts `*BIN` / `*MMDB`) with a create that watches the incarnation lifetime and, when that lifetime is canceled, stops the keep-current loop and closes the open file. BIN and MMDB keys SHALL be prefixed so they do not collide.

#### Scenario: New context reaches the wrapper
- **WHEN** plugin `New` is invoked with a context
- **THEN** the format wrapper opened for that instance is stored and bound to that context on the format table

### Requirement: Unreclaimed hash is disposed after grace
When no live `New` context remains for a wrapper configuration and grace elapses without a same-hash open, the wrapper SHALL stop its keep-current loop, close its file, and leave the singleton map. A later open with that configuration SHALL create a new wrapper. Closing a DatabaseProvider MUST still not end a wrapper that other instances (or a pending reclaim) still need.

#### Scenario: Config hash no longer used
- **WHEN** the last plugin instance using configuration hash H is gone (its `New` context is Done)
- **AND** no open with hash H occurs during grace
- **THEN** H’s keep-current loop is stopped
- **AND** H’s file handle is closed

#### Scenario: Provider Close does not dispose
- **WHEN** one DatabaseProvider is closed and another still holds the same wrapper configuration with a live `New` context
- **THEN** lookups on the remaining instance still succeed

#### Scenario: Dynamic config hash change disposes the old wrapper
- **WHEN** a wrapper is opened with configuration H1 and a context
- **AND** that context is cancelled
- **AND** a wrapper is opened with different configuration H2 and a new context
- **AND** grace for H1 elapses with no open of H1
- **THEN** reclaim logs show create H1, orphan H1, create H2, and dispose H1 after grace
- **AND** H2 lookups succeed
- **AND** H1’s keep-current loop is stopped
