## Purpose

Binds each format-wrapper singleton (one open BIN or MMDB file and its keep-current loop) to the Traefik `New` context through the reclaim lease, so a same-hash reload reclaims the wrapper and an unreclaimed hash is disposed after grace.

## Requirements

### Requirement: Wrapper open receives the New context
Opening a BIN or MMDB wrapper SHALL take the context passed to plugin `New` (the Plugin `life` context). The wrapper MUST open that hash on the **wrappers** reclaim table (`any`, caller asserts `*BIN` / `*MMDB`) with `Hooks` that close over the pointer assigned inside `create`. Sleep SHALL stop the keep-current updater by joining its ticker goroutine. Wake SHALL start that updater again after the previous stop has returned. Close SHALL join that same stop, then close the open file or reader. BIN and MMDB keys SHALL be prefixed so they do not collide. A BIN key SHALL be `bin:<catalogKey>:<hash>` and an MMDB key SHALL be `mmdb:<catalogKey>:<hash>`. `create` SHALL take no arguments and MUST NOT receive an incarnation lifetime from the table.

#### Scenario: New context reaches the wrapper
- **WHEN** plugin `New` is invoked with a context
- **THEN** the format wrapper opened for that instance is stored and bound to that Plugin `life` context on the wrappers table

#### Scenario: Reclaim key includes catalog map key
- **WHEN** a BIN wrapper is opened for catalog key `asnlite`
- **THEN** the wrappers-table key starts with `bin:asnlite:`
- **AND** the remainder is the wrapper-config hash

### Requirement: Same hash shares one wrapper and reclaims across reload
Two opens with the same wrapper configuration SHALL share one file and one keep-current loop. When the bound contexts are cancelled, Sleep SHALL join the keep-current stop so the ticker goroutine has exited before Sleep returns. When a later open uses the same configuration with a new live context before grace ends, Wake SHALL start the ticker again and the wrapper MUST stay open (same pointer). A second keep-current loop MUST NOT run while the wrapper is awake.

#### Scenario: Same-hash New after generation cancel
- **WHEN** two plugin instances share one wrapper configuration
- **AND** Traefik cancels their holder contexts and calls `New` again with the same configuration before grace ends
- **THEN** lookups on the shared wrapper still succeed
- **AND** the keep-current ticker is stopped while asleep
- **AND** Wake starts one ticker again

### Requirement: Unreclaimed hash is disposed after grace
When no live holder remains for a wrapper configuration and grace elapses without a same-hash open, the wrapper SHALL Close: join its keep-current stop, close its file, and leave the table. After Close returns, that generation SHALL stay disposed: lookups SHALL fail, and a download that finishes afterward SHALL NOT publish a new file handle. A later open with that configuration SHALL create a new wrapper. Closing a merged Lookup MUST still not end a wrapper that other instances (or a pending reclaim) still need.

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

### Requirement: Disposed generation stays disposed
The keep-current Updater SHALL join its ticker goroutine on Stop. After Stop is signaled, that Updater SHALL NOT invoke its update callback. Sleep and Close SHALL use that Stop as the only join. Stop MAY wait for an in-flight download (up to the existing HTTP GET timeout). BIN and MMDB SHALL ignore an update that arrives after Close, including when the join is missed: they SHALL NOT assign a live handle on a closed generation. `db == nil` SHALL NOT mean closed (AllowMissing may start with no file). BIN Close SHALL NOT take a mutex around Lookup. A BIN copy opened after Close SHALL be closed and removed.

#### Scenario: Delayed BIN download after Close
- **WHEN** a URL-backed BIN wrapper starts a download
- **AND** Close runs while that download is still in flight
- **AND** the download then finishes
- **THEN** Close returns only after the ticker goroutine has exited
- **AND** Lookup on that wrapper fails
- **AND** no new BIN temp copy from that late update remains in the process temp directory

#### Scenario: Delayed MMDB download after Close
- **WHEN** a URL-backed MMDB wrapper starts a download
- **AND** Close runs while that download is still in flight
- **AND** the download then finishes
- **THEN** Close returns only after the ticker goroutine has exited
- **AND** Lookup on that wrapper fails
