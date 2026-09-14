## MODIFIED Requirements

### Requirement: Wrapper open receives the New context
Opening a BIN or MMDB wrapper SHALL take the context passed to plugin `New` (the Plugin `life` context). The wrapper MUST open that hash on the **wrappers** reclaim table (`any`, caller asserts `*BIN` / `*MMDB`) with `Hooks` that close over the pointer assigned inside `create`. Sleep SHALL stop the keep-current updater. Wake SHALL start that updater again. Close SHALL stop the updater and close the open file or reader. BIN and MMDB keys SHALL be prefixed so they do not collide. A BIN key SHALL be `bin:<catalogKey>:<hash>` and an MMDB key SHALL be `mmdb:<catalogKey>:<hash>`. `create` SHALL take no arguments and MUST NOT receive an incarnation lifetime from the table.

#### Scenario: New context reaches the wrapper
- **WHEN** plugin `New` is invoked with a context
- **THEN** the format wrapper opened for that instance is stored and bound to that Plugin `life` context on the wrappers table

### Requirement: Same hash shares one wrapper and reclaims across reload
Two opens with the same wrapper configuration SHALL share one file and one keep-current loop. When the bound contexts are cancelled, Sleep SHALL stop the keep-current ticker. When a later open uses the same configuration with a new live context before grace ends, Wake SHALL start the ticker again and the wrapper MUST stay open (same pointer). A second keep-current loop MUST NOT run while the wrapper is awake.

#### Scenario: Same-hash New after generation cancel
- **WHEN** two plugin instances share one wrapper configuration
- **AND** Traefik cancels their holder contexts and calls `New` again with the same configuration before grace ends
- **THEN** lookups on the shared wrapper still succeed
- **AND** the keep-current ticker is stopped while asleep
- **AND** Wake starts one ticker again

### Requirement: Unreclaimed hash is disposed after grace
When no live holder remains for a wrapper configuration and grace elapses without a same-hash open, the wrapper SHALL Close: stop its keep-current loop, close its file, and leave the table. A later open with that configuration SHALL create a new wrapper.

#### Scenario: Config hash no longer used
- **WHEN** the last holder for configuration hash H is gone
- **AND** no open with hash H occurs during grace
- **THEN** H’s keep-current loop is stopped
- **AND** H’s file handle is closed
