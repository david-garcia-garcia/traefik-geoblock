## Purpose

Defines a keyed reclaim table that stores one value per key as `any`, survives context cancel when the same key is opened again within grace, and cancels the incarnation lifetime when it is not. The table lives in `pkg/reclaim` and is reusable across packages. Callers type-assert. Yaegi cannot instantiate `Table[T]` from another package; this table is not generic.

## Requirements

### Requirement: Table file depends only on the Go standard library
The `Table` source file SHALL import only Go standard-library packages. It MUST NOT import this module’s plugin, wrapper, source, or vendor packages. It MUST store `any`. It MUST NOT be a generic `Table[T]` instantiated as `otherpkg.Table[*T]` (Yaegi panics or fails import).

#### Scenario: Stdlib-only imports
- **WHEN** `table.go` is listed for imports
- **THEN** every import path is a Go standard-library package

### Requirement: Process table is a singleton
`pkg/reclaim` SHALL expose one process-wide table (`Default` / package `Open`). Independent keys on that table MUST NOT share an incarnation. Callers in other packages SHALL type-assert the value `Open` returns.

#### Scenario: Default Open shares one incarnation
- **WHEN** `Open` and `Default().Open` are called for the same key
- **THEN** both return the same stored value
- **AND** `create` runs once

### Requirement: Open creates once and binds a context
`Open(ctx, key, create)` SHALL create the value on the first call for a key, store it, and bind `ctx` as a holder. `create` SHALL receive a lifetime context that is canceled when this incarnation ends. If `create` runs and another Open already stored the key, the table MUST cancel that create’s lifetime immediately and MUST NOT store that value. A later `Open` for the same key (live or in grace) SHALL return the stored value, bind the new context, and MUST NOT run `create` or replace the lifetime. The lifetime SHALL be canceled at most once per incarnation. Two live contexts on one key SHALL keep the value until both are Done.

#### Scenario: Two holders one dispose
- **WHEN** `Open` creates a value for a key
- **AND** a second `Open` attaches another live context to that key
- **THEN** the lifetime is not canceled while either context is not Done

#### Scenario: Second create dispose is ignored
- **WHEN** a key already has an incarnation
- **AND** `Open` is called again
- **THEN** `create` does not run
- **AND** the original lifetime remains the one that will be canceled

### Requirement: Cancel then open within grace does not dispose
When every bound context for a key is Done, the table SHALL wait a grace period before canceling the lifetime. If the same key is opened again with a live context before grace ends, the table MUST NOT cancel the lifetime for that incarnation. That reclaim MUST NOT run `create` again.

#### Scenario: Reclaim before grace
- **WHEN** all contexts for a key are Done
- **AND** a new `Open` for that key occurs before grace ends
- **THEN** the lifetime is not canceled
- **AND** the new context is tracked
- **AND** the stored value is returned

#### Scenario: Grace elapses without rebind
- **WHEN** all contexts for a key are Done
- **AND** no `Open` for that key occurs during grace
- **THEN** the lifetime is canceled once

### Requirement: Keys are independent
Canceling the lifetime of one key MUST NOT cancel the lifetime of another key.

#### Scenario: One key times out
- **WHEN** key A’s contexts are all Done and grace elapses
- **AND** key B still has a live context
- **THEN** only key A’s lifetime is canceled

### Requirement: Grace is configurable
The table SHALL use a caller-supplied grace duration. A zero or negative grace SHALL still wait until after the Done notification is processed. Default grace in this product SHALL be 10 seconds when the caller does not pass a duration.

#### Scenario: Default grace
- **WHEN** a table is created without an explicit grace
- **THEN** grace is 10 seconds

### Requirement: Lifecycle events are logged
The table SHALL emit a structured log line for each of: incarnation created (`Open` create), holder attached, last holder gone and grace started (orphan), holder attached during grace (reclaim), and lifetime canceled. Each line MUST include the key. Message strings SHALL be stable package constants (`reclaim_put`, `reclaim_bind`, `reclaim_orphan`, `reclaim_reclaim`, `reclaim_dispose`). `reclaim_put` and `reclaim_dispose` SHALL be logged at info. The others SHALL be logged at debug.

#### Scenario: Hash change orphan then dispose
- **WHEN** key A is opened, then all of A’s contexts are Done
- **AND** key B is opened (new incarnation) before or after A’s grace starts
- **AND** A is not opened again during grace
- **THEN** logs include create A, bind A, orphan A, create B, bind B, and dispose A
- **AND** dispose A occurs only after grace for A
- **AND** B’s lifetime is not canceled
