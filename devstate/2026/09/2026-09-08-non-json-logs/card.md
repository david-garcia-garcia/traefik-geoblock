Developer review: ready for review — 2026-09-08T12:57:32Z

IssueKey: 2026-09-08-non-json-logs
JobName: 2026-09-08-non-json-logs

Upstream: [PascalMinder/geoblock#67](https://github.com/PascalMinder/geoblock/issues/67)

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** Package tests lock that GeoBlock stdout never uses the [PascalMinder/geoblock#67](https://github.com/PascalMinder/geoblock/issues/67) `INFO: GeoBlock:` prefix, that default/text lines do not start with `{`, and that `logFormat: json` lines are JSON objects. Default `logFormat` stays `text`. The observability spec now includes those line-shape SHALLs.

**End users.** None.

## Motivation
[PascalMinder/geoblock#67](https://github.com/PascalMinder/geoblock/issues/67) reported CrowdSec `crowdsecurity/traefik-logs` failing `UnmarshalJSON` on `INFO: GeoBlock:` lines. On `master` that prefix is already gone (slog `time=…`). Current hub skips non-`{` lines. DestBranch had no test that those shapes stay true.

If we do not merge the lock, a logger edit can restore the prefix or emit a `{` line that is not JSON, and CrowdSec warnings return with no failing test.

```mermaid
sequenceDiagram
  participant Issue as Issue 67 sample
  participant Master as master stdout
  participant Tests as this PR tests
  Issue->>Master: INFO: GeoBlock: …
  Note over Master: prefix already gone
  Tests->>Tests: fail if prefix or curly-brace text returns
```

## Merge readiness
Ready for review. CI succeeded. Checklist empty. Title is no longer WIP.

Priority: P3 — DestBranch already lacks the filed prefix; this PR locks that with tests.
Reviewed head: 3fb9249
Owner decision: Required. See Decision needed.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | CI green; no open comments; tests passed. |
| CI proof | 6/6 | Lint, Test, Integration Tests succeeded on [run 34228846590](https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/34228846590). |
| Local tests proof | N/A | Remote PR. `localTests: passed`. |
| Review resolution | 6/6 | No open PR review comments. |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-08-non-json-logs pushed | git `3fb9249` |
| OpenSpec | crowdsec-compatible-stdout-logs (archived) | openspec/changes/archive/2026-09-08-crowdsec-compatible-stdout-logs/ |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/81 | GitHub PR 81 |
| CI | build 34228846590 succeeded https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/34228846590 | Lint/Test/Integration Tests success |
| Local tests | passed | `go test ./... -count=1` |
| PR comments | no comments | comments: none |
| Upstream issue status comment | Set skipped | No write access to PascalMinder/geoblock#67 |

## Specs
- [core_geoblock_observability_decision-header](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-08-non-json-logs/openspec/changes/archive/2026-09-08-crowdsec-compatible-stdout-logs/proposal.md) — modified

## Follow-up issues
- [ ] [Bootstrap and owner loggers ignore `logFormat`](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-08-non-json-logs/knowledge/debt/2026-09-08-bootstrap-owner-logformat.md) — NewBootstrap and NewOwner always emit text even when logFormat is json.

## How this fits together
Upstream [PascalMinder/geoblock#67](https://github.com/PascalMinder/geoblock/issues/67) → branch 2026-09-08-non-json-logs → [PR 81](https://github.com/david-garcia-garcia/traefik-geoblock/pull/81) with green CI.

## Decision needed
| Question | Decision | By |
| --- | --- | --- |
| Should CreateConfig default `logFormat` become `json`? | assumed — no. Filed prefix is absent; current hub skips text; json can mis-tag as access | explore |
| Must `NewBootstrap` / `NewOwner` honor `logFormat`? | assumed — not this run. Noted as follow-up | explore |

## Before merge
None.

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-08-non-json-logs/devstate/2026/09/2026-09-08-non-json-logs/codereview_standards.md) — 4 total, 0 pending, 0 completed, 4 skipped
[Nitpicks](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-08-non-json-logs/devstate/2026/09/2026-09-08-non-json-logs/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-08-non-json-logs/devstate/2026/09/2026-09-08-non-json-logs/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-08-non-json-logs/devstate/2026/09/2026-09-08-non-json-logs/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-08-non-json-logs/devstate/2026/09/2026-09-08-non-json-logs/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-08-non-json-logs/devstate/2026/09/2026-09-08-non-json-logs/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-08-non-json-logs/devstate/2026/09/2026-09-08-non-json-logs/codereview_coverage.md) — 0 total, 0 pending, 0 completed

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Fold onto observability leaf; archived |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | No review comments |
| Reviewed head | 3fb924918a435105610a989053429e441a71bff3 | After archive commit |

### Stored data model
None.

### Technical review
Best possible solution: lock DestBranch line shapes with package tests; do not flip default to json.

Do we have a high-confidence way to reproduce? Yes — tests capture stdout; #67 prefix not reproduced; CI green.

Is this the best way to solve the issue? Yes versus DestBranch — the filed bug is absent; the gap was proof.

### Evidence
What I checked:
- Measured stdout 2026-09-08 (no `INFO: GeoBlock:`)
- CrowdSec hub v1.5 `startsWith "{"` guard (`knowledge/research/ext_crowdsec_parsers_traefik-logs/`)
- `go test ./...` passed; CI run 34228846590 success

### Rank-up moves
None.
