Developer review: in progress — 2026-09-08T12:50:06Z

IssueKey: 2026-09-08-non-json-logs
JobName: 2026-09-08-non-json-logs

Upstream: [PascalMinder/geoblock#67](https://github.com/PascalMinder/geoblock/issues/67)

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** Package tests lock that stdout never uses the #67 `INFO: GeoBlock:` prefix, that default/text lines do not start with `{`, and that `logFormat: json` lines are JSON objects. `CreateConfig` default `logFormat` stays `text`.

**End users.** None.

## Motivation
[PascalMinder/geoblock#67](https://github.com/PascalMinder/geoblock/issues/67) reported CrowdSec `UnmarshalJSON` failures on `INFO: GeoBlock:` lines. On `master` that prefix is already gone (slog `time=…`). Current hub skips non-`{` lines. DestBranch had no test that those shapes stay true.

If we do not merge the lock, a logger edit can restore the prefix or emit a `{` line that is not JSON, and CrowdSec warnings return with no failing test.

```mermaid
sequenceDiagram
  participant Issue as Issue 67 sample
  participant Tests as package tests
  participant Hub as CrowdSec hub v1.5
  Issue->>Hub: INFO: GeoBlock: …
  Note over Hub: issue-era UnmarshalJSON warning
  Tests->>Tests: reject that prefix and curly-brace text
  Note over Tests: json format still Unmarshal
```

## Merge readiness
Apply landed. Local `go test ./...` passed. CI on the test commit is still running. Archive and ready title remain.

Priority: P3 — DestBranch already lacks the filed prefix; this PR locks that with tests.
Reviewed head: a08b3c3
Owner decision: Required. See Decision needed.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | Tests landed; CI in progress; not archived. |
| CI proof | 3/6 | Lint, Test, Integration Tests in progress on [run 34228333880](https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/34228333880). |
| Local tests proof | N/A | Remote PR; CI is the proof axis. `localTests: passed`. |
| Review resolution | 6/6 | No open PR review comments. |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-08-non-json-logs pushed | git `a08b3c3` |
| OpenSpec | crowdsec-compatible-stdout-logs | openspec/changes/crowdsec-compatible-stdout-logs/ |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/81 | GitHub PR 81 |
| CI | build 34228333880 in progress https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/34228333880 | GitHub check runs in_progress |
| Local tests | passed | `go test ./... -count=1` exit 0 |
| PR comments | no comments | comments: none |
| Upstream issue status comment | Set skipped | No write access to PascalMinder/geoblock#67 |

## Specs
- [core_geoblock_observability_decision-header](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-08-non-json-logs/openspec/changes/crowdsec-compatible-stdout-logs/proposal.md) — modified

## Follow-up issues
- [ ] [Bootstrap and owner loggers ignore `logFormat`](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-08-non-json-logs/knowledge/debt/2026-09-08-bootstrap-owner-logformat.md) — NewBootstrap and NewOwner always emit text even when logFormat is json.

## How this fits together
Upstream [PascalMinder/geoblock#67](https://github.com/PascalMinder/geoblock/issues/67) → branch 2026-09-08-non-json-logs → PR 81 → tests on `pkg/logging` and `CreateConfig` / `PluginLogger`.

## Decision needed
| Question | Decision | By |
| --- | --- | --- |
| Should CreateConfig default `logFormat` become `json`? | assumed — no. The filed format is absent; current hub skips text; json can mis-tag as access | explore |
| Must `NewBootstrap` / `NewOwner` honor `logFormat`? | assumed — not this run. Note as follow-up | explore |

## Before merge
- [ ] Green CI on PR 81
- [ ] Archive change + drop WIP title

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Fold onto observability leaf |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | No review comments |
| Reviewed head | a08b3c32402d2b5d7e937b595fd1b680484a5610 | After apply commit |

### Stored data model
None.

### Technical review
Best possible solution: lock DestBranch line shapes with package tests; do not flip default to json.

Do we have a high-confidence way to reproduce? Yes — tests capture stdout; #67 prefix not present; `go test ./...` passed.

Is this the best way to solve the issue? Yes versus DestBranch — the filed bug is absent; the gap was proof.

### Evidence
What I checked:
- `go test ./pkg/logging/ ./pkg/geoblock/ -count=1` passed
- `go test ./... -count=1` passed
- Tasks 1.1–3.1 marked done

### Rank-up moves
None.
