## Motivation
The plugin reads files it ships with (the seed databases under `seeds/` and `geoblockban.html`) from its own folder on disk. Traefik never tells a plugin where that folder is, so the plugin read it from `TRAEFIK_PLUGIN_GEOBLOCK_PATH`, and that variable had to be one exact directory.

That broke the normal setup in four ways:

1. **Catalog installs could not be configured.** Traefik unpacks a catalog plugin to `plugins-storage/sources/gop-<random>/src/github.com/david-garcia-garcia/traefik-geoblock`, with a new random name on every start. No fixed value works, and a glob was rejected as "not a directory".
2. **Without the variable, startup failed.** The plugin did not try the folders where Traefik actually puts it; it logged an ERROR and the middleware did not load.
3. **A source with a `url` still needed a local file.** If its bundled file was missing, startup failed instead of downloading it. MMDB sources could never start without a file.
4. **Misleading logs and docs.** An ERROR "bundled file not found" appeared even when a downloaded database was already in use, and the README pointed at a folder Traefik no longer writes.

Priority: P2 — real operator pain with a workaround (set the exact path by hand on local installs; none for catalog installs).

## Implementation
**Finding the plugin folder** (`pkg/fileutils`, `PluginDir`):
- `TRAEFIK_PLUGIN_GEOBLOCK_PATH` set to an existing directory: that directory is used, as before.
- Set to anything else: it is treated as a glob. No match is an error; it does not fall back to the defaults.
- Not set: the plugin looks where Traefik puts it, relative to Traefik's working directory (`/` in the official image), the same way Traefik resolves these folders: `plugins-local/src/<module>` first, then `plugins-storage/sources/*/src/<module>`.
- Several matches: the most recently modified folder wins (Traefik unpacks the catalog plugin again on each start).
- One folder is chosen for every bundled file, so a seed and `geoblockban.html` never come from two different unpacks. The choice is cached per variable value.

**Starting without a database** (`pkg/geoblock`, `pkg/dbwrappers`): a source with a `url` starts even when no local file exists, BIN and MMDB alike. It logs one WARN `no database file yet; waiting for auto-update` with the reason, and the download runs as usual. A source without a `url` and without a file still fails to start.

**Logs:** finding a file no longer writes to the log. The error says which folder was used, where it came from (variable or Traefik default) and the exact paths tried. The caller logs it once: a WARN when the source waits for a download, otherwise it is the startup error. A missing seed when a downloaded file is already in use is DEBUG.

**Yaegi:** Traefik runs plugins in the Yaegi interpreter, not as compiled Go. Yaegi v0.16.1 mis-runs a tuple assignment such as `a, b = x, t.ModTime()`; the first version of this change used one, and the folder search found nothing inside Traefik while `go test` passed. The code now uses two statements, and CI runs the `fileutils`, `dbutils` and `logging` tests under Yaegi (`make test-yaegi`).

README, the url-download spec and the devdocs describe this behaviour.

## What this changes
**Operators.** With the official Traefik image, `TRAEFIK_PLUGIN_GEOBLOCK_PATH` is no longer needed, for local and catalog installs. It still accepts an exact directory, and now also a glob. A source with a `url` and no local file starts and downloads its database; until the download lands, lookups fail and the default `banIfError: true` blocks those requests (`banIfError: false` lets them through, but CIDR, private-network and country rules still apply). Missing-file messages name the folder and paths tried.

**Admin users.** None.

**Developers.** `fileutils.Search` and `dbsource.BundledFile` no longer take a logger and never log; callers decide the level. `fileutils.PluginDir` is the single place that locates the plugin folder. `MMDBConfig` gains `AllowMissing`. Code that Traefik runs must avoid tuple assignments that Yaegi mis-runs; `make test-yaegi` checks the packages that support it.

**End users.** Only during the first start without any local database file: requests are blocked (default) until the download finishes. In the Docker check that took under a second; without outbound internet access it never finishes.

## Merge readiness
Not ready: the CI **Test** job is red because of a flaky test that already fails on `master` (see Verification). Everything else is green.

Priority: P2 — real operator pain with a workaround
Reviewed head: 5435941
Owner decision: Required on the two choices in "Deviations from the ask" (folder order, and one folder for all files).

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| CI | 3 of 4 jobs green | Lint, Yaegi tests and Docker integration tests pass; **Test** fails on a pre-existing flaky test |
| Local tests | passed | `go test ./...`, `go vet ./...`, `golangci-lint` clean on changed files |
| Review resolution | 8 of 9 findings fixed, 1 kept by owner decision | See Findings |
| PR comments | none open | |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Unit tests | passed | `go test ./...` on the merged head |
| Yaegi tests | passed | `make test-yaegi` in `golang:1.21` (Linux); with the tuple bug put back, 5 `fileutils` tests fail there while `go test` passes |
| Official image, no variable, working directory `/` | seed opened from `/plugins-local/.../seeds/`; US IP passes, other country 403 | `traefik:v3.7.11`, local plugin, cold start |
| Official image, no variable, working directory `/work` | seed opened from `/work/plugins-local/...`; same decisions | same |
| Official image, exact path and glob in the variable | seed opened from the plugin folder; same decisions | same |
| Catalog layout | `/plugins-storage/sources/gop-<n>/src/<module>/seeds/` with working directory `/` | `traefik:v3.7.11` with the catalog plugin v1.2.1; this branch's code covered by a unit test on that layout (also run under Yaegi) |
| Conflicts with master | resolved | merge of `b1f45f2` (#97); one devdocs line kept from each side |
| CI | Lint, Test (Yaegi), Integration Tests passed; **Test failed** | [run 42038831232](https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/42038831232) on `5435941` |
| Flaky `TestNew` | fails 2/5 on `master`, 4/5 on this PR before the review fixes, 2/5 after | Linux, non-root, clean clone. `FreeCatalogKeySucceeds` or `GeoNamedKeyIsOrdinary` fail with `TempDir RemoveAll cleanup: directory not empty`: the update dir is empty when the subtest ends and a file appears afterwards, so something keeps writing after the plugin is torn down. Writer not identified yet. |

## Specs
Worktree:
- [core_geoblock_database_source-catalog](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/openspec/changes/archive/2026-11-03-plugin-path-glob/proposal.md) — modified
- [core_geoblock_database_url-download](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/openspec/changes/archive/2026-11-03-plugin-path-glob/proposal.md) — modified

Completed:
- [core_geoblock_database_source-catalog](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/openspec/specs/core_geoblock_database_source-catalog/spec.md) — modified
- [core_geoblock_database_url-download](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/openspec/specs/core_geoblock_database_url-download/spec.md) — modified (plugin folder rules and log contract updated after review)

## Deviations from the ask
| Ask | Delivered | Why |
| --- | --- | --- |
| Default paths `/plugins-storage/...` then `/plugins-local/...` (absolute) | Relative to Traefik's working directory | Traefik itself resolves `./plugins-local` and `./plugins-storage` that way. Same result in the official image (working directory `/`); also correct when Traefik runs from another directory. |
| Try the catalog folder first, then the local one | Local first, then catalog | A mounted local plugin is a deliberate choice; a leftover catalog unpack must not override it. |
| With several matches, keep the one that contains `seeds/<defaultFile>` | Most recently modified folder, used for every file | Choosing per file could take the seed and `geoblockban.html` from two different unpacks. Traefik normally leaves one `gop-*` folder, so this only matters with leftovers. |
| — (not asked) | CI job running tests under Yaegi | Needed to catch Yaegi-only bugs like the one found during review. |

## Follow-up issues
Open: the flaky `TestNew` above (pre-existing on `master`). Decision pending on whether it is fixed in this PR.

Proposed, not filed yet: run more packages under Yaegi. `dbsource` tests locate `seeds/` with `runtime.Caller`, which points at the Go install under Yaegi. `dbwrappers` (MMDB tests) and `geoblock`/`dbprovider` (a `reflect.Set` panic with test stubs) fail under Yaegi; the real lookups worked in the Docker run, but the cause is not confirmed.

## How this fits together
Ticket 2026-11-03-plugin-path-glob, branch `2026-11-03-plugin-path-glob` into `master`, PR https://github.com/david-garcia-garcia/traefik-geoblock/pull/96. CI: [run 42038831232](https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/42038831232).

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| Who owns the plugin folder path the plugin has to find? | bounded asked — 2 production callers (`BundledFile`, ban HTML in `Prepare`) | Traefik's plugin manager creates it (`plugins-storage/sources/gop-*/src/<module>`, or `plugins-local/src/<module>`) and does not pass it to the plugin. The plugin rebuilds it from the variable or from those default locations, and does not compute it any other way. | propose |
| Several matching folders: which one? | asked | One folder for all files, newest first (see Deviations). | review |

## Findings
Code review of `3de4849`, fixed in `64afdb4` unless noted.

| # | Finding | Resolution |
| --- | --- | --- |
| 1 | A source with a `url` starts with no database, so a missed plugin folder blocks all traffic until the download lands | By design (asked). The main trigger, the folder not being found, is fixed by #2 and checked in the official image. |
| 2 | Default paths were absolute; Traefik's are relative to its working directory | Fixed |
| 3 | Each file could come from a different plugin folder | Fixed: one folder for all files |
| 4 | A test depended on folders on the host machine | Fixed: defaults are swappable in tests |
| 5 | Two contradictory WARNs, neither naming the folder found | Fixed: one error that names it |
| 6 | The same failure logged three times; the waiting WARN dropped the cause | Fixed |
| 7 | Repeated glob and duplicate file checks | Fixed |
| 8 | Tests leave temp folders behind | Kept: test-only (owner decision) |
| 9 | Throwaway logger per call and a duplicate check | Fixed |
| — | Found while verifying in Docker: the folder search failed inside Traefik (Yaegi tuple assignment) | Fixed; CI Yaegi job added in `01b0fc8` |

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_standards.md) — 2 total, 0 pending, 2 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_spec.md) — 1 total, 0 pending, 1 completed
[Scope](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-plugin-path-glob/devstate/2026/11/2026-11-03-plugin-path-glob/codereview_coverage.md) — 0 total, 0 pending, 0 completed

These axis files are from the automated review of `3de4849`; the later review is in Findings.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 4 modified | Same list as Specs |
| Open reviewer comments | 0 | Unanswered review is merge risk |
| Reviewed head | 5435941a0634634e8ddf62fae850faa13d34cc71 | Card must match the branch you measured |

### Stored data model
None.
