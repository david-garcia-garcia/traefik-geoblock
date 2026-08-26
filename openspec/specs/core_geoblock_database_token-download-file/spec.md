## Purpose

Defines how token-authenticated IP2Location auto-update sets the download `file=` query from the configured package code so paid or LITE BIN downloads can succeed.

## Requirements

### Requirement: Token download uses the configured code exactly
When a download token is configured, the auto-update download URL SHALL set `file=` to `databaseAutoUpdateCode` with no transformation. The `file=` value MUST NOT be a ZIP filename such as `IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP`. The plugin MUST NOT append or map suffixes such as `BINIPV6`.

#### Scenario: Official package code is passed through
- **WHEN** a download token is set and `databaseAutoUpdateCode` is `DB8BINIPV6`
- **THEN** the download request uses `file=DB8BINIPV6`

#### Scenario: LITE package code is passed through
- **WHEN** a download token is set and `databaseAutoUpdateCode` is `DB1LITEBINIPV6`
- **THEN** the download request uses `file=DB1LITEBINIPV6`

#### Scenario: Short product is not rewritten
- **WHEN** a download token is set and `databaseAutoUpdateCode` is `DB8`
- **THEN** the download request uses `file=DB8` (not `DB8BINIPV6` and not a ZIP filename)

#### Scenario: Token URL is not a ZIP filename
- **WHEN** a download token is set and `databaseAutoUpdateCode` is `DB1`
- **THEN** the download request MUST NOT use `file=IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP`

### Requirement: Empty token keeps the free LITE URL
When no download token is configured, auto-update SHALL keep using the existing free LITE download URL. It MUST NOT send a token `file=` package code on that path.

#### Scenario: Free LITE download unchanged
- **WHEN** `databaseAutoUpdateToken` is empty
- **THEN** the download uses the free LITE ZIP URL on `download.ip2location.com`

### Requirement: On-disk names use the configured code
Auto-update SHALL name and find stored BIN files using `databaseAutoUpdateCode` as configured (`*IP2LOCATION-LITE-{code}.IPV6.BIN`).

#### Scenario: Stored file includes the configured code
- **WHEN** a token download succeeds for configured code `DB8BINIPV6`
- **THEN** the stored file name includes `IP2LOCATION-LITE-DB8BINIPV6.IPV6.BIN`
