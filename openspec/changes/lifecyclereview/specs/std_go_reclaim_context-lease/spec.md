## Purpose

Defines a stdlib-only keyed reclaim lease so a shared instance survives context cancel when a new context rebinds the same key within grace, and is disposed when it does not. Other projects can copy the package without this product’s types.

## ADDED Requirements

### Requirement: Reclaim depends only on the Go standard library
The reclaim component SHALL import only Go standard-library packages. It MUST NOT import this module’s plugin, wrapper, source, or vendor packages. It MUST NOT store caller values as `any` (Yaegi type-assert panic when the map lives in another package).

#### Scenario: Stdlib-only imports
- **WHEN** the reclaim package is listed for imports
- **THEN** every import path is a Go standard-library package

### Requirement: Bind tracks a set of contexts per key
A reclaim set SHALL accept a key, a context, and a dispose callback. Multiple binds of the same key SHALL keep one dispose callback (the first successful bind’s callback, or the latest rebound callback after a completed dispose). Dispose for that key SHALL run at most once per incarnation (create → dispose).

#### Scenario: Two contexts one key
- **WHEN** two binds use the same key with two live contexts and one dispose callback
- **THEN** that callback is not invoked while either context is not Done

### Requirement: Cancel then rebind within grace does not dispose
When every bound context for a key is Done, the set SHALL wait a grace period before invoking dispose. If the same key is bound again with a live context before grace ends, the set MUST NOT invoke dispose for that incarnation.

#### Scenario: Reclaim before grace
- **WHEN** all contexts for a key are Done
- **AND** a new bind for that key occurs before grace ends
- **THEN** dispose is not invoked
- **AND** the new context is tracked

#### Scenario: Grace elapses without rebind
- **WHEN** all contexts for a key are Done
- **AND** no bind for that key occurs during grace
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
