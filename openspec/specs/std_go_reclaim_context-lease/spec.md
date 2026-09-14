## Purpose

Defines a keyed reclaim table that stores one value per key as `any`, survives context cancel when the same key is opened again within grace, and Closes the incarnation when it is not. The table lives in `pkg/reclaim` and is reusable across packages. Callers type-assert. Yaegi cannot instantiate `Table[T]` from another package; this table is not generic.

## Requirements

### Requirement: Table file depends only on the Go standard library
The `Table` source file SHALL import only Go standard-library packages. It MUST NOT import this module’s plugin, wrapper, source, or vendor packages. It MUST store `any`. It MUST NOT be a generic `Table[T]` instantiated as `otherpkg.Table[*T]` (Yaegi panics or fails import).

#### Scenario: Stdlib-only imports
- **WHEN** `table.go` is listed for imports
- **THEN** every import path is a Go standard-library package

### Requirement: Process table is a singleton
`pkg/reclaim` SHALL NOT expose a process-wide `Default` or package `Open`. Callers SHALL construct a table with `New(Config)` and hold the `*Table`. Independent keys on one table MUST NOT share an incarnation. Callers SHALL type-assert the value `Open` returns. `NewTable`, package `Reset`, and `ResetWith` MUST NOT exist. `(*Table) Reset` MAY exist for tests only.

#### Scenario: Caller-owned New shares one incarnation
- **WHEN** the same `*Table` `Open`s the same key twice with live contexts
- **THEN** both return the same stored value
- **AND** `create` runs once

#### Scenario: Separate tables do not share
- **WHEN** two `New(Config)` tables `Open` the same key string
- **THEN** each stores its own incarnation

### Requirement: Open creates once and binds a context
`Open(ctx, key, logger, create, hooks)` SHALL create the value on the first call for a key, store `hooks` on that incarnation, and bind `ctx` as a holder. `Open` SHALL panic if `ctx` is nil. `Open` SHALL return an error if `logger` is nil, `create` is nil, the table is nil, or the table was not constructed with `New`. The table MUST NOT keep a logger of its own. A holder whose `Done` is nil (`context.Background`) SHALL be treated as live until `ctx.Err()` is set or the incarnation ends. `create` SHALL take no arguments. The table MUST NOT type-assert `Close()` on the stored `any`. A later `Open` for the same key (live or asleep) SHALL ignore its `hooks` argument. The first `Open` for an absent key SHALL register the key busy before `create` runs so concurrent first Opens share one create. A create error (or recovered create panic) SHALL unmap the key; waiters receive that error; a later Open MAY create again. `(value, nil)` means this call bound a holder that was still live at return. If `ctx.Err()` is set at bind, `Open` SHALL return `(nil, ctx.Err())` and drop that holder on the same call. Two live contexts on one key SHALL keep the value until both are Done.

#### Scenario: Two holders one dispose
- **WHEN** `Open` creates a value for a key
- **AND** a second `Open` attaches another live context to that key
- **THEN** Close does not run while either context is not Done

#### Scenario: Second create is ignored
- **WHEN** a key already has an incarnation
- **AND** `Open` is called again
- **THEN** `create` does not run

#### Scenario: Concurrent first Open shares one create
- **WHEN** two first `Open` calls for the same key run concurrently
- **THEN** `create` runs once
- **AND** both waiters receive that result

#### Scenario: Canceled bind returns the context error
- **WHEN** `Open` would bind a value
- **AND** `ctx.Err()` is already set
- **THEN** `Open` returns `(nil, ctx.Err())`
- **AND** that holder is dropped

#### Scenario: Missing context panics
- **WHEN** `Open` is called with a nil context
- **THEN** `Open` panics

#### Scenario: Nil logger is rejected
- **WHEN** `Open` is called with a nil logger
- **THEN** `Open` returns an error
- **AND** no incarnation is stored

### Requirement: Cancel then open within grace does not dispose
When every bound context for a key is Done, the table SHALL Sleep the value and wait a grace period before Close. If the same key is opened again with a live context before grace ends, the table MUST Wake that value, MUST NOT run `create`, and MUST NOT Close. Grace is how long a **sleeping** value is kept, not how long it stays live. The grace waiter SHALL be compiled `time.AfterFunc` (MUST NOT be an interpreted `select` on a timer channel). The wait MUST NOT run on the last-holder drop caller.

#### Scenario: Reclaim before grace
- **WHEN** all contexts for a key are Done
- **AND** a new `Open` for that key occurs before grace ends
- **THEN** Close does not run
- **AND** Wake runs
- **AND** the stored value is returned

#### Scenario: Grace elapses without rebind
- **WHEN** all contexts for a key are Done
- **AND** no `Open` for that key occurs during grace
- **THEN** Close runs once

### Requirement: Keys are independent
Closing one key MUST NOT Close another key.

#### Scenario: One key times out
- **WHEN** key A’s contexts are all Done and grace elapses
- **AND** key B still has a live context
- **THEN** only key A is Closed

### Requirement: Grace is configurable
The table SHALL copy `Config.Grace` at `New`. Later writes to that `Config` MUST NOT change the table. A zero grace SHALL Sleep then Close back to back (no reclaim window). A negative grace SHALL become `DefaultGrace` (10 seconds).

#### Scenario: Default grace
- **WHEN** a table is created with a negative grace
- **THEN** grace is 10 seconds

#### Scenario: Zero grace
- **WHEN** a table is created with a zero grace
- **AND** the last holder context is Done
- **THEN** Sleep and Close run without a sleeping window

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
