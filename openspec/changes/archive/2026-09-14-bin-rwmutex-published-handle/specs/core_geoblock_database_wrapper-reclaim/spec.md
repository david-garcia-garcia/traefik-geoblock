## ADDED Requirements

### Requirement: BIN published handle is mutex-serialized
The BIN wrapper SHALL protect the published vendor handle and its sibling published fields (`path`, `version`, local copy path, source path) with a read-write mutex. Close and hot-swap SHALL publish a new or nil handle under the write lock so an in-flight lookup cannot observe a torn pointer. Close SHALL wait for in-flight lookups that hold the read lock before it Closes the vendor handle it just unpublished. Hot-swap SHALL open the next file without holding the lock, then publish, then Close the previous vendor handle after 10 seconds (not immediately). Path, Version, and SourcePath SHALL take the read lock. The keep-current skip-compare SHALL read the published source path through that locked SourcePath getter. MMDB’s published-reader mutex SHALL stay on MMDB; this requirement MUST NOT move BIN onto a helper shared with MMDB.

#### Scenario: Concurrent lookup vs close
- **WHEN** `LookupRecord` runs while reclaim Close unpublishes the BIN handle
- **THEN** `LookupRecord` either completes `Get_all` on a still-open vendor handle or returns that the BIN is not open
- **AND** `Get_all` is not invoked on a nil vendor handle
- **AND** `go test -race` on `TestNew_ContextBindsWrapper` and `TestOpenBIN_HashChangeDisposesOld` does not report a race on that handle

#### Scenario: Concurrent lookup vs hot-swap
- **WHEN** `LookupRecord` runs while hot-swap publishes a new BIN handle
- **THEN** `Get_all` runs on one consistent vendor handle
- **AND** the previous handle is Closed after 10 seconds

#### Scenario: Getters share the published-handle mutex
- **WHEN** Path, Version, or SourcePath is read during hot-swap
- **THEN** the returned value is a consistent snapshot of the published fields
- **AND** the keep-current skip-compare uses SourcePath, not a raw unlocked field read
