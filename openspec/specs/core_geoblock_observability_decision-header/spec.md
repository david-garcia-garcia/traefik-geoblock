## Purpose

Defines request decision observability after legacy header and file-log config is removed. Operators read pass/block plus reason from one request header in Traefik access logs.

## Requirements

### Requirement: Decision header is the only request status header
When `logStatusDetailHeader` is set, the plugin SHALL write that request header as `{pass|block}:{phase}` using the same phase names already used for allow/block decisions. The plugin MUST NOT write a separate simple pass/block request header. The plugin MUST NOT write a remediation or action header on the response.

#### Scenario: Allowed request writes detail only
- **WHEN** `logStatusDetailHeader` is `X-Geoblock-Decision` and the request is allowed for `allowed_country`
- **THEN** the request header `X-Geoblock-Decision` is `pass:allowed_country`
- **AND** no `pass`/`block`-only request header is written by the plugin

#### Scenario: Blocked request writes detail only
- **WHEN** `logStatusDetailHeader` is `X-Geoblock-Decision` and the request is blocked for `blocked_country`
- **THEN** the request header `X-Geoblock-Decision` is `block:blocked_country`
- **AND** the response has no plugin-owned remediation/action header

#### Scenario: Empty decision header writes nothing
- **WHEN** `logStatusDetailHeader` is empty
- **THEN** the plugin writes no decision header on the request

### Requirement: Removed logging and header fields are not accepted
The plugin Config SHALL NOT include `logStatusHeader`, `logBannedRequests`, `logPath`, `fileLogBufferSizeBytes`, `fileLogBufferTimeoutSeconds`, or `remediationHeadersCustomName`. Remaining plugin logs SHALL go to the process stdout logger configured by `logLevel` and `logFormat`. The plugin MUST NOT open a log file and MUST NOT emit a dedicated blocked-request info log gated by a config flag.

#### Scenario: Blocked request is not file-logged
- **WHEN** a request is blocked
- **THEN** the plugin does not append a line to a configured log file
- **AND** observability of that block is only via `logStatusDetailHeader` when that field is set

#### Scenario: Logger stays on stdout
- **WHEN** the plugin is created with `logLevel` and `logFormat` set
- **THEN** those settings apply to the stdout slog logger
- **AND** there is no file destination for plugin logs
