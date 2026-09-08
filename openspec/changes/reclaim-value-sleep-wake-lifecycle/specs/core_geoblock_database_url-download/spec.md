## ADDED Requirements

### Requirement: The update loop stops deterministically and can be restarted
Stopping a source's update loop SHALL NOT return until that loop has finished. A stopped update
loop SHALL NOT perform a further age check, download, file write, or update callback, including
from a check that was already scheduled when the stop was requested. Stopping SHALL be safe to
call more than once and safe to call on a source that was never started. After a stop returns,
starting the same source again SHALL run a normal update loop with no goroutine left over from
the previous run.

#### Scenario: Stop waits for the update loop
- **WHEN** an update loop is running for a source
- **AND** that source is stopped
- **THEN** stopping does not return until the loop has finished
- **AND** no file is written into the source directory afterwards

#### Scenario: A stopped loop does not download
- **WHEN** a source is stopped before its first age check has run
- **THEN** no download is performed for that source
- **AND** no update callback is invoked

#### Scenario: Stop then start runs a fresh loop
- **WHEN** a source's update loop is stopped and the source is started again
- **THEN** the loop runs again
- **AND** only one update loop exists for that source

#### Scenario: Stopping twice is safe
- **WHEN** a source is stopped and then stopped again
- **THEN** neither call panics
- **AND** the loop is still finished
