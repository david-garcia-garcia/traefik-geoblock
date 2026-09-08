## ADDED Requirements

### Requirement: Incarnation end closes the stored value before it reports the end
When an incarnation ends (grace elapsed while sleeping, `Reset`, or a zero-grace drop), the table
SHALL call `Close()` on the stored value and SHALL wait until it has returned before it emits
`reclaim_dispose`. `Close()` SHALL be called at most once per incarnation. Because the table
waits, a value whose `Close()` blocks blocks whoever ended the incarnation; values stored on this
table SHALL NOT block in `Close()`. Every goroutine the table starts for a key SHALL exit once
that key's holder contexts are Done and its incarnation has ended.

#### Scenario: Dispose log implies Close has returned
- **WHEN** a key is orphaned and grace elapses
- **AND** the stored value has a `Close()` method
- **THEN** `Close()` has returned before `reclaim_dispose` is emitted for that key
- **AND** `Close()` did not observe a dispose line already written for that key

#### Scenario: Reset closes the value before it reports dispose
- **WHEN** `Reset` is called on a table that still has an incarnation
- **THEN** that value's `Close()` has returned before `reclaim_dispose` is emitted for that key

#### Scenario: Goroutines do not outlive the incarnation
- **WHEN** many keys are opened, then every holder context is Done and every incarnation has ended
- **THEN** the table owns no more goroutines than it did before those `Open` calls

## MODIFIED Requirements

### Requirement: Open creates once and binds a context
`Open(ctx, key, logger, create)` SHALL create the value on the first call for a key, store it,
and bind `ctx` as a holder. `Open` SHALL panic if `ctx` is nil. `Open` SHALL return an error if
`logger` is nil. The table MUST NOT keep a logger of its own; `logger` is the only logger for
that Open. A holder whose `Done` is nil (`context.Background`) SHALL be treated as live until
`ctx.Err()` is set. `create` SHALL take no arguments (Yaegi cannot call
`func(context.Context) (any, error)`). The table SHALL register the key before it runs `create`,
so concurrent first `Open` calls for one key run `create` exactly once and every caller receives
that one value; no `Open` SHALL create a value that is then discarded. If `create` returns an
error the key SHALL NOT be stored, every caller waiting on that create SHALL receive that error,
and a later `Open` SHALL be free to try again. A later `Open` for the same key (live or sleeping)
SHALL return the stored value, bind the new context, and MUST NOT run `create`. Two live contexts
on one key SHALL keep the value until both are Done. A stale holder drop from a previous
incarnation or from `Reset` MUST NOT change a later incarnation of the same key.

#### Scenario: Two holders one dispose
- **WHEN** `Open` creates a value for a key
- **AND** a second `Open` attaches another live context to that key
- **THEN** the incarnation does not end while either context is not Done

#### Scenario: Second create dispose is ignored
- **WHEN** a key already has an incarnation
- **AND** `Open` is called again
- **THEN** `create` does not run
- **AND** the stored value is the one returned

#### Scenario: Concurrent first Opens run create once
- **WHEN** two first `Open` calls for the same key race
- **THEN** `create` runs exactly once
- **AND** both calls return that one value
- **AND** no value is created and then discarded

#### Scenario: Create error reaches every waiting caller
- **WHEN** two first `Open` calls for the same key race and `create` returns an error
- **THEN** both calls return that error
- **AND** nothing is stored for that key
- **AND** a later `Open` for that key runs `create` again

#### Scenario: Missing context panics
- **WHEN** `Open` is called with a nil context
- **THEN** `Open` panics

#### Scenario: Nil logger is rejected
- **WHEN** `Open` is called with a nil logger
- **THEN** `Open` returns an error
- **AND** no incarnation is stored

### Requirement: Cancel then open within grace does not dispose
When every bound context for a key is Done, the table SHALL put the stored value to sleep and
keep it for a grace period before it disposes of it. If the same key is opened again with a live
context before grace ends, the table SHALL wake that value and MUST NOT dispose of it. That
reclaim MUST NOT run `create` again.

#### Scenario: Reclaim before grace
- **WHEN** all contexts for a key are Done
- **AND** a new `Open` for that key occurs before grace ends
- **THEN** the incarnation is not disposed
- **AND** the new context is tracked
- **AND** the stored value is returned awake

#### Scenario: Grace elapses without rebind
- **WHEN** all contexts for a key are Done
- **AND** no `Open` for that key occurs during grace
- **THEN** the incarnation is disposed once

### Requirement: Grace is configurable
Grace SHALL be how long a **sleeping** value is kept before it is disposed. Because a sleeping
value has released what is expensive to hold idle, a long grace is cheap: the reason to keep it
long is that a sleeping value costs little, not that reloads are fast. The table SHALL use a
caller-supplied grace duration. A zero grace SHALL dispose of the value as soon as the last
holder is gone, with no sleeping window at all. A negative grace SHALL become the product default
of 10 seconds. Default grace in this product SHALL be 10 seconds (`DefaultGrace`) when the
process table is constructed.

#### Scenario: Default grace
- **WHEN** a table is created with a negative grace
- **THEN** grace is 10 seconds

#### Scenario: Zero grace
- **WHEN** a table is created with a zero grace
- **AND** the last holder context is Done
- **THEN** the value is slept and disposed without waiting
- **AND** the key is not left stored

### Requirement: Lifecycle events are logged
The table SHALL emit a structured log line for each of: incarnation created (`Open` create),
holder attached, last holder gone and the value put to sleep (orphan), a sleeping value woken by
an `Open` (reclaim), and incarnation disposed. Each line MUST include the key. Message strings
SHALL be stable package constants (`reclaim_put`, `reclaim_bind`, `reclaim_orphan`,
`reclaim_reclaim`, `reclaim_dispose`). All five messages SHALL be logged at debug. Put, bind, and
reclaim SHALL use the logger passed to the `Open` that caused them. Orphan and dispose SHALL use
the logger from the last `Open` that bound that key. `Reset` SHALL emit `reclaim_dispose` for
each disposed key using that slot's last Open logger, preceded by `reclaim_orphan` when that
incarnation was still awake. Log lines MUST NOT be emitted while the table mutex is held.

Every line SHALL mean the work it names has already happened: `reclaim_orphan` is emitted after
`sleep` has returned, `reclaim_reclaim` after `wake` has returned, and `reclaim_dispose` after
`Close()` has returned. For one incarnation `reclaim_orphan` SHALL always precede the
`reclaim_dispose` that ends it, at every grace duration including zero, with no exception for
`Reset`.

At zero grace the table keeps no sleeping value, so an `Open` that races the last holder going
away SHALL be logged as a new incarnation (`reclaim_put` then `reclaim_bind`), not as
`reclaim_reclaim`.

#### Scenario: Hash change orphan then dispose
- **WHEN** key A is opened, then all of A's contexts are Done
- **AND** key B is opened (new incarnation) before or after A is put to sleep
- **AND** A is not opened again during grace
- **THEN** logs include create A, bind A, orphan A, create B, bind B, and dispose A
- **AND** dispose A occurs only after grace for A
- **AND** B is not disposed

#### Scenario: Orphan precedes dispose at a short grace
- **WHEN** a table with a grace shorter than a millisecond has its last holder for a key go Done
- **AND** the grace elapses and the incarnation is disposed
- **THEN** the recorded order for that key is `reclaim_orphan` then `reclaim_dispose`

#### Scenario: Zero grace open racing the drop is a plain bind
- **WHEN** a table with zero grace has its last holder for a key go Done
- **AND** an `Open` for that key races that drop
- **THEN** that `Open` records `reclaim_put` and `reclaim_bind` for the key
- **AND** it does not record `reclaim_reclaim`

#### Scenario: Reset logs orphan then dispose
- **WHEN** `Reset` is called on a table that still has a live incarnation
- **THEN** logs include `reclaim_orphan` then `reclaim_dispose` for that key
- **AND** a later `Open` of the same key creates a new incarnation that a stale holder drop MUST NOT dispose

#### Scenario: Open logger level gates put and dispose
- **WHEN** `Open` is called with a logger whose handler level is debug
- **THEN** `reclaim_put` is emitted at debug
- **WHEN** that incarnation is later disposed
- **THEN** `reclaim_dispose` is emitted at debug
- **WHEN** `Open` is called with a logger whose handler level is info
- **THEN** `reclaim_put` and `reclaim_dispose` are not emitted
