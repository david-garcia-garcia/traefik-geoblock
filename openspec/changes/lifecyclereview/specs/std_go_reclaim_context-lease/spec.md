## Purpose

Defines a stdlib-only keyed reclaim lease so a shared instance survives context cancel when a new context rebinds the same key within grace, and is disposed when it does not. Other projects can copy the package without this product’s types.

## ADDED Requirements

### Requirement: Reclaim depends only on the Go standard library
The reclaim component SHALL import only Go standard-library packages. It MUST NOT import this module’s plugin, wrapper, source, or vendor packages. It MUST NOT store caller values as `any` (Yaegi type-assert panic when the map lives in another package).

#### Scenario: Stdlib-only imports
- **WHEN** the reclaim package is listed for imports
- **THEN** every import path is a Go standard-library package

### Requirement: Put owns dispose; Bind only attaches a context
A reclaim set SHALL split incarnation setup from leases. `Put` registers one dispose callback for a key’s current incarnation (the creator). `Bind` attaches one live context as a holder. Holders MUST NOT supply dispose. A second `Put` for a key that still has an incarnation (live binds or grace pending) MUST NOT replace dispose. Dispose SHALL run at most once per incarnation (Put → dispose). `Bind` without a prior `Put` for that incarnation SHALL fail.

#### Scenario: Two holders one dispose
- **WHEN** `Put` registers dispose for a key
- **AND** two `Bind` calls attach two live contexts to that key
- **THEN** dispose is not invoked while either context is not Done

#### Scenario: Second Put is ignored
- **WHEN** a key already has an incarnation
- **AND** `Put` is called again with a different dispose
- **THEN** the original dispose remains the one that will run

### Requirement: Cancel then rebind within grace does not dispose
When every bound context for a key is Done, the set SHALL wait a grace period before invoking dispose. If the same key is bound again with a live context before grace ends, the set MUST NOT invoke dispose for that incarnation. That reclaim `Bind` MUST NOT require a new `Put`.

#### Scenario: Reclaim before grace
- **WHEN** all contexts for a key are Done
- **AND** a new `Bind` for that key occurs before grace ends
- **THEN** dispose is not invoked
- **AND** the new context is tracked

#### Scenario: Grace elapses without rebind
- **WHEN** all contexts for a key are Done
- **AND** no `Bind` for that key occurs during grace
- **THEN** dispose is invoked once

### Requirement: Keys are independent
Dispose of one key MUST NOT dispose another key.

#### Scenario: One key times out
- **WHEN** key A’s contexts are all Done and grace elapses
- **AND** key B still has a live context
- **THEN** only key A’s dispose is invoked

### Requirement: Grace is configurable
The set SHALL use a caller-supplied grace duration. A zero or negative grace SHALL still wait until after the Done notification is processed (no dispose on the same call that observes the last Done without a rebind window). Default grace in this product SHALL be 10 seconds when the caller does not pass a duration.

#### Scenario: Default grace
- **WHEN** a set is created without an explicit grace
- **THEN** grace is 10 seconds

### Requirement: Lifecycle events are logged
The set SHALL emit a structured log line for each of: incarnation created (`Put`), holder attached (`Bind`), last holder gone and grace started (orphan), holder attached during grace (reclaim), and dispose. Each line MUST include the key. Message strings SHALL be stable package constants so a test can assert the sequence without scraping free text.

#### Scenario: Hash change orphan then dispose
- **WHEN** key A is Put and Bound, then all of A’s contexts are Done
- **AND** key B is Put and Bound (new incarnation) before or after A’s grace starts
- **AND** A is not Bound again during grace
- **THEN** logs include create A, bind A, orphan A, create B, bind B, and dispose A
- **AND** dispose A occurs only after grace for A
- **AND** B’s dispose is not invoked
