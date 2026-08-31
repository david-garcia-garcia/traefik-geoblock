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
`Open(ctx, key, logger, create)` SHALL create the value on the first call for a key, store it, and bind `ctx` as a holder. `Open` SHALL panic if `ctx` is nil. `Open` SHALL return an error if `logger` is nil. The table MUST NOT keep a logger of its own; `logger` is the only logger for that Open. A holder whose `Done` is nil (`context.Background`) SHALL be treated as live until `ctx.Err()` is set. `create` SHALL take no arguments (Yaegi cannot call `func(context.Context) (any, error)`). If the stored value has `Close()`, the table SHALL call it when this incarnation ends. If `create` runs and another Open already stored the key, the table MUST cancel that create’s lifetime immediately and MUST NOT store that value. A later `Open` for the same key (live or in grace) SHALL return the stored value, bind the new context, and MUST NOT run `create` or replace the lifetime. The lifetime SHALL be canceled at most once per incarnation. Two live contexts on one key SHALL keep the value until both are Done. A stale holder drop from a previous incarnation or from `Reset` MUST NOT change a later slot for the same key.

#### Scenario: Two holders one dispose
- **WHEN** `Open` creates a value for a key
- **AND** a second `Open` attaches another live context to that key
- **THEN** the lifetime is not canceled while either context is not Done

#### Scenario: Second create dispose is ignored
- **WHEN** a key already has an incarnation
- **AND** `Open` is called again
- **THEN** `create` does not run
- **AND** the original lifetime remains the one that will be canceled

#### Scenario: Lost create race
- **WHEN** two first `Open` calls for the same key run `create` concurrently
- **THEN** both return the stored value
- **AND** the losing create’s lifetime is canceled
- **AND** that losing value is not stored

#### Scenario: Missing context panics
- **WHEN** `Open` is called with a nil context
- **THEN** `Open` panics

#### Scenario: Nil logger is rejected
- **WHEN** `Open` is called with a nil logger
- **THEN** `Open` returns an error
- **AND** no incarnation is stored

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
The table SHALL use a caller-supplied grace duration. A zero grace SHALL cancel the lifetime as soon as the last holder is gone (no wait). A negative grace SHALL become the product default of 10 seconds. Default grace in this product SHALL be 10 seconds (`DefaultGrace`) when the process table is constructed.

#### Scenario: Default grace
- **WHEN** a table is created with a negative grace
- **THEN** grace is 10 seconds

#### Scenario: Zero grace
- **WHEN** a table is created with a zero grace
- **AND** the last holder context is Done
- **THEN** the lifetime is canceled without waiting

### Requirement: Lifecycle events are logged
The table SHALL emit a structured log line for each of: incarnation created (`Open` create), holder attached, last holder gone and grace started (orphan), holder attached during grace (reclaim), and lifetime canceled. Each line MUST include the key. Message strings SHALL be stable package constants (`reclaim_put`, `reclaim_bind`, `reclaim_orphan`, `reclaim_reclaim`, `reclaim_dispose`). All five messages SHALL be logged at debug. Put, bind, and reclaim SHALL use the logger passed to the `Open` that caused them. Orphan and dispose SHALL use the logger from the last `Open` that bound that key. `Reset` SHALL emit `reclaim_dispose` for each canceled key using that slot’s last Open logger. Log lines MUST NOT be emitted while the table mutex is held.

#### Scenario: Hash change orphan then dispose
- **WHEN** key A is opened, then all of A’s contexts are Done
- **AND** key B is opened (new incarnation) before or after A’s grace starts
- **AND** A is not opened again during grace
- **THEN** logs include create A, bind A, orphan A, create B, bind B, and dispose A
- **AND** dispose A occurs only after grace for A
- **AND** B’s lifetime is not canceled

#### Scenario: Reset logs dispose
- **WHEN** `Reset` is called on a table that still has an incarnation
- **THEN** logs include `reclaim_dispose` for that key
- **AND** a later `Open` of the same key creates a new incarnation that a stale holder drop MUST NOT cancel

#### Scenario: Open logger level gates put and dispose
- **WHEN** `Open` is called with a logger whose handler level is debug
- **THEN** `reclaim_put` is emitted at debug
- **WHEN** that incarnation is later disposed
- **THEN** `reclaim_dispose` is emitted at debug
- **WHEN** `Open` is called with a logger whose handler level is info
- **THEN** `reclaim_put` and `reclaim_dispose` are not emitted

