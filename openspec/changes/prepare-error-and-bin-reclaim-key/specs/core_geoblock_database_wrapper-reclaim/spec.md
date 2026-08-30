## MODIFIED Requirements

### Requirement: Wrapper open receives the New context
Opening a BIN or MMDB wrapper SHALL take the context passed to plugin `New`. The plugin MUST pass that context through catalog source open into the wrapper open. The wrapper MUST open that hash on the process `reclaim` table (`any`, caller asserts `*BIN` / `*MMDB`) with a create that watches the incarnation lifetime and, when that lifetime is canceled, stops the keep-current loop and closes the open file. BIN and MMDB keys SHALL be prefixed so they do not collide. A BIN key SHALL be `bin:<catalogKey>:<hash>` and an MMDB key SHALL be `mmdb:<catalogKey>:<hash>`, where `catalogKey` is the `databaseSources` map key and `hash` is the wrapper-config hash.

#### Scenario: New context reaches the wrapper
- **WHEN** plugin `New` is invoked with a context
- **THEN** the format wrapper opened for that instance is stored and bound to that context on the format table

#### Scenario: Reclaim key includes catalog map key
- **WHEN** a BIN wrapper is opened for catalog key `asnlite`
- **THEN** the process-table key starts with `bin:asnlite:`
- **AND** the remainder is the wrapper-config hash
