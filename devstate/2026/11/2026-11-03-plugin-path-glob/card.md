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
Reviewed head: 68b4bab
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
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/96 | pr-host |
| CI | not seen | caller omitted CI snapshot |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-03-plugin-path-glob on branch 2026-11-03-plugin-path-glob targeting master; PR https://github.com/david-garcia-garcia/traefik-geoblock/pull/96; CI not seen.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Who already owns the on-disk plugin-root path that `Search` reconstructs from `TRAEFIK_PLUGIN_GEOBLOCK_PATH`? | bounded asked — existing Search contract (must be a directory); 2 production call sites enumerated (`BundledFile`, `Prepare` ban HTML), roots `pkg/**`; criterion 1 names find the plugin folder without a fixed path | assumed — Traefik’s plugin Manager owns `GoPath()` (`os.MkdirTemp(sourcesRootPath, "gop-*")` then `src/<module>`). That value is not given to plugin code. Reconstruct it from the operator env (exact directory still works) or the asked default globs that match Manager layout and `./plugins-local/`. Do not re-derive it from a peer library or from Yaegi internals. | explore |
| When several glob matches all contain `seeds/<defaultFile>`, or two catalog rows pass different `defaultFile` names, which plugin root wins? | additive asked — new disambiguation this change adds; criterion 1 names several matches keep the one containing `seeds/<defaultFile>` | assumed — first sorted Glob match that contains `seeds/<that call’s defaultFile>`, else `<defaultFile>` at root, else not-found. Search stays per-call (ban HTML uses `geoblockban.html`). Two rows picking two roots is accepted; Traefik normally has one `gop-*`. | explore |
| BundledFile missing-seed while a dated file is already the resolved path: DEBUG or WARN? | additive asked — log-level change on an existing miss path; criterion 3 names DEBUG or WARN | assumed — DEBUG. The operator already has a working dated file; WARN implies action. `initialize`’s return remains the fatal signal when there is no URL. | explore |
| What operator contract is required for `banIfError: false` while the database is missing? | additive asked — documentation of an existing branch; criterion 2 names decide and document; Out of scope: changing the `BanIfError` default | assumed — default stays true (fail-closed until a handle exists). `false` is fail-open on lookup errors only: `ServeHTTP` does not ban for `lookupFailed` / empty country, then `decide()` still applies CIDR, private, country lists, and `defaultAllow`. Document that in README and url-download. Do not add a new bypass flag. | explore |
| Exact on-disk `gop-*` shape versus the example glob (`*` vs `gop-*`)? | additive asked — README wording; criterion 4 names `gop-*` docs; Out of scope: inventing a layout other than the named glob, local fallback, and `gop-*` docs | assumed — keep the asked default glob `*` (it matches Traefik `MkdirTemp(..., "gop-*")`). README names the folder `gop-*` under `/plugins-storage/sources` and the full `{gop-*}/src/github.com/david-garcia-garcia/traefik-geoblock` shape. Do not change the glob to `gop-*` only. | explore |


## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 68b4babb946df2998ff9a1af2f76386a6662fe9d | Card must match the branch you measured |

### Stored data model
None.
