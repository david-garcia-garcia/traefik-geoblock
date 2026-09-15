# Bootstrap and owner loggers ignore `logFormat`

IssueKey: 2026-09-08-non-json-logs
Size: small
Action: note

## Why this follow-up

`logging.New` honors `logFormat`. `NewBootstrap` and `NewOwner` always use `slog.NewTextHandler`. Operators who set `logFormat: json` still get text setup and wrapper lines on stdout.

## Why it was not taken

This run’s job is to prove the #67 `INFO: GeoBlock:` prefix is gone and that json format is valid JSON. Current CrowdSec hub skips lines that do not start with `{`. Changing bootstrap/owner is extra.

## Risks

Old CrowdSec hub collections without the `startsWith "{"` guard still UnmarshalJSON those text lines and warn.
