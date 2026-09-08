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
`Open(ctx, key, logger, grace, create)` SHALL create the value on the first call for a key, store it, and bind `ctx` as a holder. `Open` SHALL panic if `ctx` is nil. `Open` SHALL return an error if `logger` is nil. The table MUST NOT keep a logger of its own; `logger` is the only logger for that Open. `grace` is the grace for the incarnation this call may create, and is governed by the grace requirement below. A holder whose `Done` is nil (`context.Background`) SHALL be treated as live until `ctx.Err()` is set. `create` SHALL take no arguments (Yaegi cannot call `func(context.Context) (any, error)`). If the stored value has `Close()`, the table SHALL call it when this incarnation ends. If `create` runs and another Open already stored the key, the table MUST cancel that create’s lifetime immediately and MUST NOT store that value. A later `Open` for the same key (live or in grace) SHALL return the stored value, bind the new context, and MUST NOT run `create` or replace the lifetime. The lifetime SHALL be canceled at most once per incarnation. Two live contexts on one key SHALL keep the value until both are Done. A stale holder drop from a previous incarnation or from `Reset` MUST NOT change a later slot for the same key.

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
Grace SHALL belong to the incarnation, not to the table. The `Open` that creates a value SHALL fix that incarnation's grace from its `grace` argument; a negative `grace` (spelled `TableGrace`) SHALL mean the table's grace. When two first `Open` calls race, the grace of the create whose value is stored applies; the losing create's grace is discarded with its value. An `Open` that binds or reclaims an existing incarnation MUST NOT change that incarnation's grace, whatever it passes. Two keys on one table MAY therefore have different graces. The table's grace is the default for every `Open` that names none: it is supplied at `NewTable`, and a negative value there SHALL become the product default of 10 seconds (`DefaultGrace`), which is what the process table is constructed with. A zero grace SHALL cancel the lifetime as soon as the last holder is gone (no wait).

#### Scenario: Default grace
- **WHEN** a table is created with a negative grace
- **THEN** grace is 10 seconds

#### Scenario: Zero grace
- **WHEN** a table is created with a zero grace
- **AND** the last holder context is Done
- **THEN** the lifetime is canceled without waiting

#### Scenario: One key names its own grace
- **WHEN** two keys are opened on one table, one with a zero grace and one with `TableGrace`
- **AND** both holder contexts are Done
- **THEN** the zero-grace key's lifetime is canceled without waiting
- **AND** the other key's lifetime is not canceled before the table's grace elapses

#### Scenario: A key outlives a table that ends its keys immediately
- **WHEN** a key is opened with a positive grace on a table whose grace is zero
- **AND** the holder context is Done
- **THEN** that key's lifetime is not canceled before its own grace elapses

#### Scenario: Reclaim does not change the grace
- **WHEN** a key is created by an `Open` naming one grace
- **AND** a later `Open` reclaims that incarnation naming a different grace
- **THEN** the incarnation keeps the grace of the `Open` that created it

### Requirement: Lifecycle events are logged
The table SHALL emit a structured log line for each of: incarnation created (`Open` create), holder attached, last holder gone and grace started (orphan), holder attached during grace (reclaim), and lifetime canceled. Each line MUST include the key. Message strings SHALL be stable package constants (`reclaim_put`, `reclaim_bind`, `reclaim_orphan`, `reclaim_reclaim`, `reclaim_dispose`). All five messages SHALL be logged at debug. Put, bind, and reclaim SHALL use the logger passed to the `Open` that caused them. Orphan and dispose SHALL use the logger from the last `Open` that bound that key. `Reset` SHALL emit `reclaim_dispose` for each canceled key using that slot’s last Open logger. Log lines MUST NOT be emitted while the table mutex is held. `reclaim_orphan` SHALL be emitted before the grace period starts. So for one incarnation `reclaim_orphan` always precedes the `reclaim_dispose` that ends grace, at every grace duration including zero, and always precedes the `reclaim_reclaim` of an `Open` that lands in that grace window. Two narrow cases are outside that ordering, and both are stated so a reader does not treat them as defects: a `Reset` cancels every mapped incarnation at once, so it MAY emit `reclaim_dispose` before an orphan line that a concurrent last-holder cancel has not finished writing; and an `Open` that binds in the instant between the last holder going and the orphan line being written MAY record its `reclaim_reclaim` before that line.

#### Scenario: Hash change orphan then dispose
- **WHEN** key A is opened, then all of A’s contexts are Done
- **AND** key B is opened (new incarnation) before or after A’s grace starts
- **AND** A is not opened again during grace
- **THEN** logs include create A, bind A, orphan A, create B, bind B, and dispose A
- **AND** dispose A occurs only after grace for A
- **AND** B’s lifetime is not canceled

#### Scenario: Orphan precedes dispose at a short grace
- **WHEN** a table with a grace shorter than a millisecond has its last holder for a key go Done
- **AND** the grace elapses and the lifetime is canceled
- **THEN** the recorded order for that key is `reclaim_orphan` then `reclaim_dispose`

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

#### Scenario: Orphan and dispose use the last binding logger
- **WHEN** a key is opened with logger one, then opened again with logger two while the incarnation is live
- **AND** every holder context is later Done and grace elapses
- **THEN** `reclaim_orphan` and `reclaim_dispose` for that key are emitted through logger two
- **AND** logger one records no `reclaim_orphan` or `reclaim_dispose` for that key

### Requirement: Incarnation end closes the stored value before it reports the end
When an incarnation ends (grace elapsed while orphaned, `Reset`, or a lost create race), the table SHALL cancel the incarnation lifetime and then wait until the stored value's `Close()` has returned before it reports the end. For the two paths that report it — grace elapsed and `Reset` — the table SHALL emit `reclaim_dispose` only after that `Close()` has returned. A lost create race closes the value it created inline and emits no `reclaim_dispose`, because that value was never stored. `Close()` SHALL be called at most once per incarnation. Because the table waits, a value whose `Close()` blocks blocks whoever ended the incarnation; values stored on this table SHALL NOT block in `Close()`. Every goroutine the table starts for a key SHALL exit once that key's holder contexts are Done and its incarnation has ended.

#### Scenario: Dispose log implies Close has returned
- **WHEN** a key is orphaned and grace elapses
- **AND** the stored value has a `Close()` method
- **THEN** `Close()` has returned before `reclaim_dispose` is emitted for that key

#### Scenario: Reset closes the value before it reports dispose
- **WHEN** `Reset` is called on a table that still has an incarnation
- **THEN** that value's `Close()` has returned before `reclaim_dispose` is emitted for that key
- **AND** `Reset` does not return before that dispose is emitted

#### Scenario: Goroutines do not outlive the incarnation
- **WHEN** many keys are opened, then every holder context is Done and every incarnation has ended
- **THEN** the table owns no more goroutines than it did before those `Open` calls

### Requirement: Either side of the grace edge is correct
An `Open` that races the end of grace for the same key SHALL either reclaim the stored incarnation (returning the stored value and keeping the lifetime) or bind a new incarnation created by that `Open`. Both outcomes are correct. In neither outcome SHALL the table cancel the lifetime of an incarnation that has a live holder, and in neither outcome SHALL a value be left stored after its lifetime was canceled.

#### Scenario: Open racing grace expiry
- **WHEN** the last holder for a key is Done and grace is armed
- **AND** an `Open` for that key runs concurrently with the grace expiry
- **THEN** the returned value is either the stored incarnation or a newly created one
- **AND** the incarnation the returned value belongs to has a live holder and is not canceled

