## Context

See proposal.md — Why. Current hub `crowdsecurity/traefik-logs` (v1.5) unmarshals only lines that trim-start with `{`. Text slog (`time=…`) is skipped. slog JSON unmarshals but can be mis-tagged as an access event. `pkg/logging` already implements both formats; tests check substrings only.

## Goals / Non-Goals

**Goals:**
- Lock measured line shapes with package tests next to `pkg/logging` and `CreateConfig`.
- Capture real stdout (`StdoutWriter` / `os.Stdout`) so the assert matches Traefik process logs.

**Non-Goals:**
- Changing default `logFormat`.
- Passing format into `NewBootstrap` / `NewOwner` (debt).
- Emitting Traefik access-log JSON fields.
- Changing CrowdSec Hub.

## Decisions

- **Assert bytes, not CrowdSec in-process.** Alternatives: vendor the hub parser (heavy, version-pin). Capture stdout and apply the same predicates the parser uses (`startsWith "{"`, `json.Unmarshal`).
- **Keep default text.** Alternatives: default json (stops issue-era UnmarshalJSON, but current hub then treats plugin JSON as a candidate access line). Research: text is the safe mixed-stream shape on current hub.
- **Fold onto `core_geoblock_observability_decision-header`.** FindSpecHost: small adjustment (one–three requirements) to the leaf that already owns stdout `logLevel` / `logFormat`. Alternatives: new leaf `stdout-lines` (extra family leaf for one invariant).

## Risks / Trade-offs

- [Issue-era CrowdSec without `startsWith "{"`] → Mitigation: text still fails UnmarshalJSON on those collections; operators can set `logFormat: json` or split access logs. Not this change.
- [os.Stdout capture races] → Mitigation: reuse the pipe pattern already in `logging_test.go`; one writer at a time per test.

## Migration Plan

None. Tests only. Rollback is revert the test/spec commit.
