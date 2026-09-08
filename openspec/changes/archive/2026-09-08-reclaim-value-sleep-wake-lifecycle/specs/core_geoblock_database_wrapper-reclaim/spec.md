## ADDED Requirements

### Requirement: An unheld wrapper sleeps instead of staying fully live
While a BIN or MMDB wrapper is stored but held by nobody, it SHALL NOT keep its update loop
running. When the last holder of a wrapper's context is Done, the wrapper SHALL stop its update
loop and SHALL NOT return until that loop has finished, so no download or file write can happen
after it has been put to sleep. When the wrapper is opened again before its grace ends, it SHALL
restart the update loop and SHALL be usable for lookups when `Open` returns. Sleeping and waking
SHALL NOT change the wrapper's identity: the same wrapper is returned.

#### Scenario: An orphaned wrapper stops its update loop
- **WHEN** every holder of a wrapper's context is Done and the wrapper has been put to sleep
- **THEN** the wrapper's update loop is no longer running
- **AND** no further download for that source is started

#### Scenario: A woken wrapper serves lookups and updates again
- **WHEN** a sleeping wrapper is opened again before its grace ends
- **THEN** the same wrapper is returned
- **AND** a lookup against it succeeds
- **AND** its update loop is running again

#### Scenario: Disposal does not stop the update loop a second time
- **WHEN** a sleeping wrapper's grace elapses and it is disposed
- **THEN** the wrapper releases its database handle
- **AND** it does not stop an update loop that sleeping already stopped
