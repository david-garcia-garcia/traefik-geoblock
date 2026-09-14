## Purpose

Defines the stored-value lifecycle on the reclaim table: create, sleep, wake, and close via `Hooks`, without type-switching the stored `any`.

## Requirements

### Requirement: Stored-value events use Hooks
One stored value SHALL move create → (sleep → wake)* → sleep → close. Create and Close run at most once per incarnation. Sleep and Wake are a matched pair. Close SHALL always be preceded by Sleep (including zero grace and `Reset` of an awake value), except a Sleep panic which still Closes and skips orphan. `Hooks` fields `Sleep`, `Wake`, and `Close` are optional `func()`; a nil field skips that event. The table MUST NOT type-switch or assert the stored `any` for those events. Callers SHALL pass funcs that close over a pointer assigned inside `create`. `EnforceCloseBeforeOpen` is stored at put; default false unmaps then Close; true keeps the key mapped until Close returns.

#### Scenario: Last holder sleeps then closes
- **WHEN** the last holder for a key is Done
- **AND** grace elapses with no reclaim
- **THEN** Sleep runs
- **AND** then Close runs

#### Scenario: Reclaim wakes the sleeper
- **WHEN** a value is asleep in grace
- **AND** `Open` is called for that key
- **THEN** Wake runs
- **AND** the same pointer is returned
- **AND** Close does not run

#### Scenario: Nil Sleep is skipped
- **WHEN** `Hooks.Sleep` is nil
- **AND** the last holder is Done
- **THEN** the value is still parked asleep for grace
- **AND** Close still runs after grace (or immediately when grace is zero)

### Requirement: Hook panics are recovered
Create, Sleep, Wake, and Close panics SHALL be recovered. A create or Wake panic SHALL be returned as an error to that Open (waiters receive the same error). A Sleep or Close panic with no caller SHALL log `reclaim_hook_panic` at error (`key`, `hook`). Close panics MUST NOT kill an AfterFunc goroutine.

#### Scenario: Create panic unmaps
- **WHEN** `create` panics
- **THEN** `Open` returns a wrapped error
- **AND** the key is not stored
- **AND** Close is not called
