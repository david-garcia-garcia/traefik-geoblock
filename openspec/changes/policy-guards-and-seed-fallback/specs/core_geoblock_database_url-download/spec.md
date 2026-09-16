## ADDED Requirements

### Requirement: Unreadable dated file falls back to seed
When the newest dated `YYYYMMDD_<catalogKey>` file exists and opening it fails, plugin creation MUST NOT fail if catalog `path` is an existing readable file or `defaultFile` resolves. The wrapper SHALL warn, open that seed, and start keep-current. A later tick MAY retry the dated file; a failed hot-swap SHALL leave the seed published. BIN and MMDB SHALL share this initialize fallback. The dated file MUST NOT be deleted or renamed by this fallback.

#### Scenario: Corrupt dated BIN opens the catalog path
- **WHEN** the auto-update dir contains an unreadable dated BIN for the catalog key
- **AND** catalog `path` is a valid BIN
- **THEN** plugin creation succeeds
- **AND** the live handle is the seed
- **AND** a warning names the dated file

#### Scenario: Corrupt dated MMDB opens the catalog path
- **WHEN** the auto-update dir contains an unreadable dated MMDB for the catalog key
- **AND** catalog `path` is a valid MMDB
- **THEN** plugin creation succeeds
- **AND** the live handle is the seed
- **AND** a warning names the dated file

#### Scenario: Traefik New survives a corrupt dated file
- **WHEN** `databaseAutoUpdateDir` contains an unreadable dated file for the enabled row
- **AND** that row's `path` is a valid seed
- **THEN** `New` succeeds
