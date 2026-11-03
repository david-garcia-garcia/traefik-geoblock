## Motivation
The plugin reconstructs the on-disk plugin directory from `TRAEFIK_PLUGIN_GEOBLOCK_PATH` (`Search`) so `seeds/<defaultFile>` and `geoblockban.html` can be joined. Catalog open then decides whether a missing file is fatal or a wait-for-GET. Traefik unpacks catalog plugins under `{plugins-storage}/sources/gop-<random>/src/<module>` and does not hand that path to plugin code.

`Search` required an existing directory. A glob such as `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock` failed as "is not a directory". Unset env logged ERROR `TRAEFIK_PLUGIN_GEOBLOCK_PATH must be set to the plugin root` with no try of that glob or `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock`. Several matches had no rule for which directory contained `seeds/<defaultFile>`.

A catalog row with a URL still failed New when `defaultFile` was missing. BIN `AllowMissing` was true only when `Path` and `DefaultFileName` were both empty. MMDB hardcoded `allowMissing: false`. `initialize` returned `failed to resolve database path` / `database file not found` before keep-current could GET. The live url-download spec required that fail. `banIfError: false` while the handle is nil already skipped the lookup ban and continued into `decide()`, with no operator contract that this is fail-open on lookup only.

`chooseStartupTarget` still called `BundledFile` after a dated file had already won, and `Search` logged ERROR `bundled file not found; TRAEFIK_PLUGIN_GEOBLOCK_PATH is probably not the plugin root`. README Local plugin still named `/plugins-storage/sources/github.com/david-garcia-garcia/traefik-geoblock` with no `gop-*`.

Left alone, catalog deploys cannot find seeds without knowing the random `gop-*` name. URL-backed reserved rows never start keep-current. Operators treat a working dated open as a broken plugin root. Docs point at a path Traefik no longer writes.

Priority: P2 — real operator, admin-user, or end-user pain, with a workaround or limited blast radius

## Implementation
`Search` is the seam for plugin-directory location. An exact env directory wins and skips glob. A set env (glob or exact) is exclusive: `filepath.Glob` matches that are directories, zero matches do not fall through. Unset env tries `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock` then `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock`. Several matches keep the first lexicographic directory that contains `seeds/<that call's defaultFile>`, else `<defaultFile>` at that directory.

`openCatalogRow` sets allow-missing when the row has a URL or empty path and empty defaultFile. `MMDBConfig` gains `AllowMissing`. `initialize` WARNs `no database file yet; waiting for auto-update` and returns so keep-current still runs. Missing `defaultFile` with no URL still fails New. Nil-handle lookups stay `BIN is not open` / `MMDB is not open`.

`Search` WARNs a bundled-file miss that is the only remaining path (tried `seeds/` and root paths, env probably not the plugin directory) and WARNs an exclusive glob or directory miss. `chooseStartupTarget` peeks `BundledFile` only when `DefaultFileName` is set and DEBUGs a miss when Latest already won.

README names Traefik's `gop-*` unpack and documents `banIfError: false` as fail-open on lookup only. The usage packet and the url-download / source-catalog delta specs follow that Search and wait contract.

## What this changes
**Operators.** `TRAEFIK_PLUGIN_GEOBLOCK_PATH` may be a glob or omitted (tries `/plugins-storage/sources/*/src/github.com/david-garcia-garcia/traefik-geoblock` then `/plugins-local/src/github.com/david-garcia-garcia/traefik-geoblock`; a set env does not fall through); a URL-backed catalog row starts with WARN `no database file yet; waiting for auto-update`; bundled miss after a dated file won is DEBUG; `banIfError` default stays true (false is fail-open on lookup only).

**Admin users.** None.

**Developers.** `Search` locates the plugin directory from an exact env path, an env glob, or those unset defaults; catalog URL wait (BIN and MMDB `AllowMissing`) starts with no on-disk file; a missing `defaultFile` with no URL still fails New.

**End users.** Until that download lands, lookups error and the default `banIfError: true` bans those requests.

## Merge readiness
In progress. 0 items remain.

Priority: P2 — real operator, admin-user, or end-user pain, with a workaround or limited blast radius
Reviewed head: 6d437c8
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
| Local tests | passed | handoff.yaml localTests |
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
[Standards](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_standards.md) — 2 total, 0 pending, 2 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_spec.md) — 1 total, 0 pending, 1 completed
[Scope](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_coverage.md) — 0 total, 0 pending, 0 completed


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 2 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 6d437c8a850cd9296aae740490887ecf0f8bc89d | Card must match the branch you measured |

### Stored data model
None.
