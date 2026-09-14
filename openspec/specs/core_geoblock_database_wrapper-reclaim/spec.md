## Purpose

Binds each format-wrapper singleton (one open BIN or MMDB file and its keep-current loop) to the Traefik `New` context through the reclaim lease, so a same-hash reload reclaims the wrapper and an unreclaimed hash is disposed after grace.

## Requirements

### Requirement: Wrapper open receives the New context
Opening a BIN or MMDB wrapper SHALL take the context passed to plugin `New` (the Plugin `life` context). The wrapper MUST open that hash on the **wrappers** reclaim table (`any`, caller asserts `*BIN` / `*MMDB`) with `Hooks` that close over the pointer assigned inside `create`. Sleep SHALL stop the keep-current updater. Wake SHALL start that updater again. Close SHALL stop the updater and close the open file or reader. BIN and MMDB keys SHALL be prefixed so they do not collide. A BIN key SHALL be `bin:<catalogKey>:<hash>` and an MMDB key SHALL be `mmdb:<catalogKey>:<hash>`. `create` SHALL take no arguments and MUST NOT receive an incarnation lifetime from the table.

#### Scenario: New context reaches the wrapper
- **WHEN** plugin `New` is invoked with a context
- **THEN** the format wrapper opened for that instance is stored and bound to that Plugin `life` context on the wrappers table

#### Scenario: Reclaim key includes catalog map key
- **WHEN** a BIN wrapper is opened for catalog key `asnlite`
- **THEN** the wrappers-table key starts with `bin:asnlite:`
- **AND** the remainder is the wrapper-config hash

### Requirement: Same hash shares one wrapper and reclaims across reload
Two opens with the same wrapper configuration SHALL share one file and one keep-current loop. When the bound contexts are cancelled, Sleep SHALL stop the keep-current ticker. When a later open uses the same configuration with a new live context before grace ends, Wake SHALL start the ticker again and the wrapper MUST stay open (same pointer). A second keep-current loop MUST NOT run while the wrapper is awake.

#### Scenario: Same-hash New after generation cancel
- **WHEN** two plugin instances share one wrapper configuration
- **AND** Traefik cancels their holder contexts and calls `New` again with the same configuration before grace ends
- **THEN** lookups on the shared wrapper still succeed
- **AND** the keep-current ticker is stopped while asleep
- **AND** Wake starts one ticker again

### Requirement: Unreclaimed hash is disposed after grace
When no live holder remains for a wrapper configuration and grace elapses without a same-hash open, the wrapper SHALL Close: stop its keep-current loop, close its file, and leave the table. A later open with that configuration SHALL create a new wrapper. Closing a merged Lookup MUST still not end a wrapper that other instances (or a pending reclaim) still need.

#### Scenario: Config hash no longer used
- **WHEN** the last holder for configuration hash H is gone
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

### Requirement: BIN published handle is mutex-serialized
The BIN wrapper SHALL protect the published vendor handle and its sibling published fields (`path`, `version`, local copy path, source path) with a read-write mutex. Close and hot-swap SHALL publish a new or nil handle under the write lock so an in-flight lookup cannot observe a torn pointer. Close SHALL wait for in-flight lookups that hold the read lock before it Closes the vendor handle it just unpublished. Hot-swap SHALL open the next file without holding the lock, then publish, then Close the previous vendor handle after 10 seconds (not immediately). Path, Version, and SourcePath SHALL take the read lock. The keep-current skip-compare SHALL read the published source path through that locked SourcePath getter. MMDB’s published-reader mutex SHALL stay on MMDB; this requirement MUST NOT move BIN onto a helper shared with MMDB.

#### Scenario: Concurrent lookup vs close
- **WHEN** `LookupRecord` runs while reclaim Close unpublishes the BIN handle
- **THEN** `LookupRecord` either completes `Get_all` on a still-open vendor handle or returns that the BIN is not open
- **AND** `Get_all` is not invoked on a nil vendor handle
- **AND** `go test -race` on `TestNew_ContextBindsWrapper` and `TestOpenBIN_HashChangeDisposesOld` does not report a race on that handle

#### Scenario: Concurrent lookup vs hot-swap
- **WHEN** `LookupRecord` runs while hot-swap publishes a new BIN handle
- **THEN** `Get_all` runs on one consistent vendor handle
- **AND** the previous handle is Closed after 10 seconds

#### Scenario: Getters share the published-handle mutex
- **WHEN** Path, Version, or SourcePath is read during hot-swap
- **THEN** the returned value is a consistent snapshot of the published fields
- **AND** the keep-current skip-compare uses SourcePath, not a raw unlocked field read
