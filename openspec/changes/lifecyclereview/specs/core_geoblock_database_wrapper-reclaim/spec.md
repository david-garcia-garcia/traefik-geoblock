## Purpose

Binds each format-wrapper singleton (one open BIN or MMDB file and its keep-current loop) to the Traefik `New` context through the reclaim lease, so a same-hash reload reclaims the wrapper and an unreclaimed hash is disposed after grace.

## ADDED Requirements

### Requirement: Wrapper open receives the New context
Opening a BIN or MMDB wrapper SHALL take the context passed to plugin `New`. The plugin MUST pass that context through the provider constructor into the wrapper open. The wrapper MUST bind its config-hash key to the reclaim set with a dispose that stops the keep-current loop and closes the open file.

#### Scenario: New context reaches the wrapper
- **WHEN** plugin `New` is invoked with a context
- **THEN** the format wrapper opened for that instance is bound to that context on the reclaim set

### Requirement: Same hash shares one wrapper and reclaims across reload
Two opens with the same wrapper configuration SHALL share one file and one keep-current loop. When the bound contexts are cancelled and a later open uses the same configuration with a new live context before grace ends, the wrapper MUST stay open and the keep-current loop MUST keep running.

#### Scenario: Same-hash New after generation cancel
- **WHEN** two plugin instances share one wrapper configuration
- **AND** Traefik cancels their `New` contexts and calls `New` again with the same configuration before grace ends
- **THEN** lookups on the shared wrapper still succeed
- **AND** a second keep-current loop is not started

### Requirement: Unreclaimed hash is disposed after grace
When no live `New` context remains for a wrapper configuration and grace elapses without a same-hash open, the wrapper SHALL stop its keep-current loop, close its file, and leave the singleton map. A later open with that configuration SHALL create a new wrapper. Closing a DatabaseProvider MUST still not dispose a wrapper that other instances (or a pending reclaim) still need.

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
