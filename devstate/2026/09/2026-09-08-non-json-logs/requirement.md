# Requirement
IssueKey: 2026-09-08-non-json-logs

## Problem

Operators who send Traefik process stdout (access logs plus plugin output) to CrowdSec’s `crowdsecurity/traefik-logs` parser see parse failures on GeoBlock lines. The parser expects JSON or Traefik CLF access-log lines; GeoBlock’s default stdout logs are plain text, so `UnmarshalJSON` errors appear when both streams share one file.

## Current (code)

- `pkg/geoblock/config.go` — `CreateConfig` sets `LogFormat` default `"text"`.
- `pkg/logging/logging.go` — `New` uses `slog.NewJSONHandler` when `format == "json"`, else `slog.NewTextHandler`; `NewBootstrap` and `NewOwner` always use `slog.NewTextHandler`.
- `pkg/geoblock/plugin.go` — `PluginLogger` calls `logging.New(name, cfg.LogLevel, cfg.LogFormat, bootstrap)`; bootstrap comes from `logging.NewBootstrap` (text).
- `README.md` — documents `logFormat: json | text`; example shows `logFormat: json`.
- `openspec/specs/core_geoblock_observability_decision-header/spec.md` — plugin logs go to stdout via `logLevel` / `logFormat`.
- Issue example lines (`INFO: GeoBlock: …`) — not found in current tree (current text output is slog key=value, not that prefix).

## Desired

GeoBlock stdout logs SHALL be compatible with CrowdSec’s Traefik log parser when operators combine plugin logs with Traefik access logs in one stream — i.e. emit JSON (or another format the parser accepts per Traefik access-log docs) instead of plain text that breaks JSON unmarshaling.

## Affected

- `pkg/logging/logging.go` — handler selection for bootstrap/owner vs main plugin logger.
- `pkg/geoblock/config.go` — default and validation of `LogFormat`.
- `pkg/geoblock/plugin.go` — logger wiring at plugin init.
- `README.md` — operator guidance for CrowdSec / shared-log setups.

## Out of scope

- Changing CrowdSec Hub `crowdsecurity/traefik-logs` to skip non-JSON lines (reporter’s alternate fix; upstream CrowdSec).
- Traefik access-log configuration or separate log files (operator wiring workaround).
- PascalMinder/geoblock repo ownership / issue routing.

## Unknowns

- Whether the fix is default `logFormat: json`, making bootstrap/owner respect `logFormat`, documenting operator config only, or a new format.
- Exact JSON field shape CrowdSec’s parser requires for non-access plugin lines (issue cites Traefik access-log JSON docs; plugin slog JSON may differ).
- Whether CLF is required or JSON alone satisfies CrowdSec for mixed streams.

## Tensions

- Ticket “expected behavior” asks the CrowdSec parser to ignore non-JSON lines; this repo can only change GeoBlock emission, not CrowdSec.
- `logFormat: json` already exists but default is `text`; issue may reflect operators who never set json, or bootstrap text lines even when json is set.
- Issue filed on upstream `PascalMinder/geoblock#67`; work proceeds on fork `david-garcia-garcia/traefik-geoblock`.
- Issue log sample (`INFO: GeoBlock:`) does not match current slog text format — may be an older release or different middleware name prefix.
