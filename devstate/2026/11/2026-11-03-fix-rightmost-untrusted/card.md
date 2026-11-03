## Motivation
Operators who put this plugin behind one or more reverse proxies need one evaluated hop: the original client, not the load balancer. Dest only offers `CheckAll` / `CheckFirst` / `CheckFirstNonePrivate`, which concatenate every present `ipHeaders` entry, read the first field line, and drop repeated IPs while keeping the first occurrence.

That collection lets a client pick the country. Repeating `8.8.8.8` on both sides of a forged `1.1.1.1` in `X-Forwarded-For` hides the real rightmost untrusted hop after first-occurrence dedup. A second `X-Forwarded-For` field line is invisible to `Header.Get`. Two configured headers on one request are walked as one concatenated list, so `X-Real-IP` can win over `X-Forwarded-For`.

Left alone, a `RightmostUntrusted` strategy that reused `GetRemoteIPs` would ship the Traefik `excludedIPs` shape without the guarantee a client cannot choose the country. Dest has no such strategy yet; CheckAll is a different contract (every hop, including proxies).

Priority: P2 — real operator pain selecting the client hop behind proxies, with CheckAll as a different-semantics workaround

## Implementation
Hop collection is split: `collectHops` reads headers and `cleanIPAddress` with no dedup; `GetRemoteIPs` stays `dedupeFirst(collectHops(..., false))` for the three existing strategies. `RightmostUntrusted` uses `clientHops` → `rightmostChain` (first forwarding header with hops, every field line, `remoteAddress` last when listed) then `rightmostUntrusted` (walk from the right, skip `trustedProxies` on a dedicated `iplookup.Helper`). Empty result reuses the existing empty-hop / `banIfError` path. `Prepare` rejects `trustedProxies` with any other strategy and requires `remoteAddress` last when listed.

## What this changes
**Operators.** Opt-in `ipHeaderStrategy: RightmostUntrusted` and `trustedProxies`. Pairing `trustedProxies` with CheckAll / CheckFirst / CheckFirstNonePrivate fails creation. Default `ipHeaders` stay valid; put `remoteAddress` last so a direct untrusted peer ignores forged headers. Each proxy must overwrite or strip configured headers it does not set.
**Admin users.** None.
**Developers.** `Config.TrustedProxies` and `IPHeaderStrategyRightmostUntrusted` are public. `GetRemoteIPs` remains first-line Get plus first-occurrence dedup; the new strategy must not call it.
**End users.** When operators enable this strategy, geo allow/block follows the rightmost untrusted hop, not a forged left-side IP.

## Merge readiness
Ready for review. 0 items remain.

Priority: P2 — real operator pain selecting the client hop behind proxies, with CheckAll as a different-semantics workaround
Reviewed head: a661da0
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | Ready |
| CI proof | 6/6 | succeeded https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/41712634951 |
| Local tests proof | N/A | remote PR — CI proof covers this |
| Review resolution | 6/6 | no open PR comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-11-03-fix-rightmost-untrusted pushed | `git` |
| OpenSpec | fix-rightmost-untrusted | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-geoblock/pull/97 | pr-host |
| CI | build 41712634951 succeeded https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/41712634951 | https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/41712634951 |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | devstate/comments.md |

## Specs
Worktree:
- [core_geoblock_plugin_ip-header-strategy](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-fix-rightmost-untrusted/openspec/changes/archive/2026-11-03-fix-rightmost-untrusted/proposal.md) — added

Completed:
- [core_geoblock_plugin_ip-header-strategy](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-fix-rightmost-untrusted/openspec/specs/core_geoblock_plugin_ip-header-strategy/spec.md) — added


## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Ticket 2026-11-03-fix-rightmost-untrusted on branch 2026-11-03-fix-rightmost-untrusted targeting master; PR https://github.com/david-garcia-garcia/traefik-geoblock/pull/97; CI build 41712634951 succeeded https://github.com/david-garcia-garcia/traefik-geoblock/actions/runs/41712634951.

## Explore Decisions
None.

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-fix-rightmost-untrusted/devstate/2026/11/2026-11-03-fix-rightmost-untrusted/codereview_standards.md) — 1 total, 0 pending, 0 completed, 1 skipped
[Nitpicks](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-fix-rightmost-untrusted/devstate/2026/11/2026-11-03-fix-rightmost-untrusted/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-fix-rightmost-untrusted/devstate/2026/11/2026-11-03-fix-rightmost-untrusted/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Scope](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-fix-rightmost-untrusted/devstate/2026/11/2026-11-03-fix-rightmost-untrusted/codereview_scope.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-fix-rightmost-untrusted/devstate/2026/11/2026-11-03-fix-rightmost-untrusted/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-fix-rightmost-untrusted/devstate/2026/11/2026-11-03-fix-rightmost-untrusted/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-fix-rightmost-untrusted/devstate/2026/11/2026-11-03-fix-rightmost-untrusted/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-geoblock/blob/2026-11-03-fix-rightmost-untrusted/devstate/2026/11/2026-11-03-fix-rightmost-untrusted/codereview_coverage.md) — 1 total, 0 pending, 1 completed


## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 2 added / 0 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | a661da0f5433e711dc2ffa2c5c3a28907218824e | Card must match the branch you measured |

### Stored data model
None.
