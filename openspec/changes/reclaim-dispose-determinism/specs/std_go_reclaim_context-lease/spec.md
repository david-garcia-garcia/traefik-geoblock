## ADDED Requirements

### Requirement: Incarnation end closes the stored value before it reports the end
When an incarnation ends (grace elapsed while orphaned, `Reset`, or a lost create race), the table SHALL cancel the incarnation lifetime, then wait until the stored value's `Close()` has returned, and only then emit `reclaim_dispose`. `Close()` SHALL be called at most once per incarnation. Because the table waits, a value whose `Close()` blocks blocks whoever ended the incarnation; values stored on this table SHALL NOT block in `Close()`. Every goroutine the table starts for a key SHALL exit once that key's holder contexts are Done and its incarnation has ended.

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

## MODIFIED Requirements

### Requirement: Lifecycle events are logged
The table SHALL emit a structured log line for each of: incarnation created (`Open` create), holder attached, last holder gone and grace started (orphan), holder attached during grace (reclaim), and lifetime canceled. Each line MUST include the key. Message strings SHALL be stable package constants (`reclaim_put`, `reclaim_bind`, `reclaim_orphan`, `reclaim_reclaim`, `reclaim_dispose`). All five messages SHALL be logged at debug. Put, bind, and reclaim SHALL use the logger passed to the `Open` that caused them. Orphan and dispose SHALL use the logger from the last `Open` that bound that key. `Reset` SHALL emit `reclaim_dispose` for each canceled key using that slot’s last Open logger. Log lines MUST NOT be emitted while the table mutex is held. `reclaim_orphan` SHALL be emitted before the grace period starts, so for one incarnation `reclaim_orphan` always precedes `reclaim_dispose` and always precedes the `reclaim_reclaim` of an `Open` that lands in that grace window, at every grace duration including zero.

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
