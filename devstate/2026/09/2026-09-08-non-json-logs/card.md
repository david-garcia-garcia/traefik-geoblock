Developer review: in progress — 2026-09-08T12:47:32Z

IssueKey: 2026-09-08-non-json-logs
JobName: 2026-09-08-non-json-logs

Upstream: [PascalMinder/geoblock#67](https://github.com/PascalMinder/geoblock/issues/67)

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** OpenSpec change `crowdsec-compatible-stdout-logs` folds stdout line-shape scenarios onto `core_geoblock_observability_decision-header`. Tests not applied yet.

**End users.** None.

## Motivation
[PascalMinder/geoblock#67](https://github.com/PascalMinder/geoblock/issues/67) filed CrowdSec `UnmarshalJSON` failures on `INFO: GeoBlock:` lines. On `master` that prefix is gone (slog `time=…` or json). Current hub skips non-`{` lines. DestBranch has no test that those shapes stay true.

If we do not merge the lock, a logger edit can restore the prefix or emit a `{` line that is not JSON, and CrowdSec warnings return with no failing test.

```mermaid
sequenceDiagram
  participant Issue as Issue 67 sample
  participant Master as master stdout
  participant Hub as CrowdSec hub v1.5
  Issue->>Hub: INFO: GeoBlock: …
  Note over Hub: issue-era UnmarshalJSON warning
  Master->>Hub: time=… slog text
  Note over Hub: startsWith curly-brace is false; skip JSON
```

## Merge readiness
Proposal is apply-ready. Implement has not landed tests. Integration Tests still running.

Priority: P3 — DestBranch already lacks the filed prefix; this PR must lock that with tests.
Reviewed head: dff3242
Owner decision: Required. See Decision needed.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | Specs written; apply not done; Integration Tests in progress. |
| CI proof | 3/6 | Lint and Test succeeded; Integration Tests in progress on [run 34227983981](https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/34227983981). |
| Local tests proof | N/A | Before implement (`none`). |
| Review resolution | 6/6 | No open PR review comments. |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-08-non-json-logs pushed | git `dff3242` |
| OpenSpec | crowdsec-compatible-stdout-logs | openspec/changes/crowdsec-compatible-stdout-logs/ |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/81 | GitHub PR 81 |
| CI | build 34227983981 in progress https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/34227983981 | Lint success, Test success, Integration Tests in_progress |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | comments: none |
| Upstream issue status comment | Set skipped | No write access to PascalMinder/geoblock#67 |

## Specs
- [core_geoblock_observability_decision-header](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-08-non-json-logs/openspec/changes/crowdsec-compatible-stdout-logs/proposal.md) — modified

## Follow-up issues
- [ ] [Bootstrap and owner loggers ignore `logFormat`](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-09-08-non-json-logs/knowledge/debt/2026-09-08-bootstrap-owner-logformat.md) — NewBootstrap and NewOwner always emit text even when logFormat is json.

## How this fits together
Upstream [PascalMinder/geoblock#67](https://github.com/PascalMinder/geoblock/issues/67) → branch 2026-09-08-non-json-logs → PR 81 → change `crowdsec-compatible-stdout-logs`.

## Decision needed
| Question | Decision | By |
| --- | --- | --- |
| Should CreateConfig default `logFormat` become `json`? | assumed — no. The filed format is absent; current hub skips text; json can mis-tag as access | explore |
| Must `NewBootstrap` / `NewOwner` honor `logFormat`? | assumed — not this run. Note as follow-up | explore |

## Before merge
- [ ] [P3] Apply tasks: line-shape tests + CreateConfig default assert
- [ ] Green CI on PR 81 (Integration Tests still running)

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
| Reviewed head | dff32421ef935a40ae2d0865fd1405767ec4fd58 | After propose commit |

### Stored data model
None.

### Technical review
Best possible solution: lock DestBranch line shapes with package tests; do not flip default to json (current hub would then ingest plugin JSON as a candidate access line).

Do we have a high-confidence way to reproduce? Yes — measured stdout 2026-09-08; #67 prefix not reproduced. Hub v1.5 sourced in `knowledge/research/ext_crowdsec_parsers_traefik-logs/`.

Is this the best way to solve the issue? Yes versus DestBranch — the filed bug is absent; the gap is proof.

### Evidence
What I checked:
- FindSpecHost fold `core_geoblock_observability_decision-header` (high)
- Research notes: hub@ce8e034 `startsWith "{"` guard
- OpenSpec 4/4 artifacts complete

### Rank-up moves
None.
