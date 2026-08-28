## MODIFIED Requirements

### Requirement: Provider implementations are isolated
Vendor Lookup SHALL live in that vendor’s provider package. Open and hot-swap SHALL live in one shared format-wrapper package (`pkg/dbwrappers`), not copied into each vendor package. HTTP GET, archive unpack, file-date read, lock, ticker, dated write, and file-location Resolve SHALL stay one shared source component (`pkg/dbsource`). A later vendor MUST be addable by implementing DatabaseProvider, adding a `databaseProvider` branch, pointing at a catalog entry, and using the BIN or MMDB format wrapper. The plugin MUST NOT type-assert a format wrapper to read provider state.

#### Scenario: Implemented vendors
- **WHEN** this change ships
- **THEN** the IP2Location provider exists
- **AND** the IPinfo Lite provider exists
- **AND** the MaxMind provider exists

#### Scenario: Plugin does not use a wrapper type
- **WHEN** the plugin looks up a public IP
- **THEN** it calls DatabaseProvider Lookup only
- **AND** it does not type-assert a BIN or MMDB wrapper

## ADDED Requirements

### Requirement: Open and hot-swap are per format
The format-wrapper package SHALL expose one BIN wrapper and one MMDB wrapper. IPinfo and MaxMind SHALL open through the MMDB wrapper. IP2Location SHALL open geo and ASN through the BIN wrapper (two instances). The same wrapper configuration SHALL share one open file and one download ticker. Closing a DatabaseProvider MUST NOT close that shared wrapper.

#### Scenario: IPinfo and MaxMind share the MMDB wrapper
- **WHEN** `databaseProvider` is `ipinfo` or `maxmind` and plugin creation opens the catalog or bundled MMDB
- **THEN** that file is opened through the shared MMDB wrapper
- **AND** the vendor package only maps lookup fields onto Record

#### Scenario: IP2Location uses two BIN wrappers
- **WHEN** `databaseProvider` is `ip2location` and geo and ASN files are resolved
- **THEN** geo and ASN are opened through the shared BIN wrapper
- **AND** the IP2Location provider maps geo Lookup plus ASN onto one Record

#### Scenario: Same config shares one wrapper
- **WHEN** two plugin instances are created with the same provider config
- **THEN** both Lookups succeed
- **AND** closing one DatabaseProvider does not make the other Lookup fail
