## Motivation
Not yet.

## Implementation
Not yet.

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None.

**End users.** None.

## Merge readiness
In progress. 0 items remain.

Priority: unknown — motivation not written
Reviewed head: d53cec9
Owner decision: Required. See Explore Decisions.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 1/6 | Not ready |
| CI proof | 1/6 | not seen |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-03-plugin-path-glob pushed | `git` |
| OpenSpec | plugin-path-glob | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/96 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [core_geoblock_database_source-catalog](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/openspec/changes/plugin-path-glob/proposal.md) — modified
- [core_geoblock_database_url-download](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/openspec/changes/plugin-path-glob/proposal.md) — modified


## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-03-plugin-path-glob on branch 2026-11-03-plugin-path-glob targeting master; PR https://github.com/david-garcia-garcia/traefik-geoblock/pull/96; CI not seen.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Who already owns the on-disk plugin-root path that `Search` reconstructs from `TRAEFIK_PLUGIN_GEOBLOCK_PATH`? | bounded asked — existing Search contract (must be a directory); 2 production call sites enumerated (`BundledFile`, `Prepare` ban HTML), roots `pkg/**`; criterion 1 names find the plugin folder without a fixed path | assumed — Traefik’s plugin Manager owns `GoPath()` (`os.MkdirTemp(sourcesRootPath, "gop-*")` then `src/<module>`). That value is not given to plugin code. Reconstruct it from the operator env (exact directory still works) or the asked default globs that match Manager layout and `./plugins-local/`. Do not re-derive it from a peer library or from Yaegi internals. | propose |


## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 2 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | d53cec99a594c772fb8a9d9c45a04340b8ba8a30 | Card must match the branch you measured |

### Stored data model
None.
