## Context

See proposal.md — Why. `blockFromHeader` walks the hops selected by `ipHeaderStrategy` and calls `decide` per hop; the deny was gated on `passReason == PhaseNone`, which the first allowed hop set.

## Goals / Non-Goals

- A hop after the first can deny, as `core_geoblock_plugin_request-mode` already requires.
- The `pass:{reason}` decision header keeps naming the first allowing phase.
- No new configuration surface.
- **Non-goal:** `decide` answers private and loopback hops from `allowPrivate` before consulting the CIDR lists, so `allowedIPBlocks` cannot allow a private hop. Pre-existing; documented in the README here, left for a separate change.

## Decisions

- **Delete the guard; do not restructure the loop.** Returning on the first deny is enough: deny is absorbing, so continuing would only change which phase is logged. Alternative: collect every hop's verdict then decide — rejected; behaviourally identical, costs lookups after the answer is known, and moves which hop the ban page names.
- **No opt-out flag.** Alternative: a flag defaulting to today's behaviour — rejected; it would ship the bypass as the default indefinitely, and defaulting it to the new behaviour makes it a lever with no user. The remedy for the affected configuration is an existing option.
- **`banIfError` in the loop is ungated too.** `ServeHTTP` already bans chain-wide on `lookupFailed && banIfError` in `enrichandblock`, so leaving the in-loop ban hop-0-gated would keep `block` mode disagreeing with `enrichandblock` on the same chain.

## Risks / Trade-offs

- The single `countryHeader` value is now applied to every selected hop, so a hop exempted by `allowedIPBlocks` no longer exempts the hops after it: `allowedIPBlocks: ["8.8.8.0/24"]` + `blockedCountries: ["US"]` on chain `8.8.8.8, 1.1.1.1` now denies at hop 1 under hop 0's `US` label. This follows the existing requirement that country allow/block use only the `countryHeader` value, and is pinned by a test rather than left to inference. A later hop's *own* country is still never evaluated; that is #66's design and is unchanged here.
- `CreateConfig` sets `CheckAll` and leaves `allowPrivate` false, so a chain whose selected hops include a private address changes from pass to `403 block:allow_private`. Mitigation: `allowPrivate: true`, or `ipHeaderStrategy: CheckFirstNonePrivate`.

## Migration Plan

Ship. For chains of well-formed addresses, operators relying on an earlier allowed hop vouching for the chain reproduce the previous verdict with `ipHeaderStrategy: CheckFirst`; an `enrich` middleware on `CheckAll` chained to a `block` middleware on `CheckFirst` also reproduces the previous `countryHeader` value. Neither is exact for a chain containing an unparseable token, where `enrich` under `CheckAll` sets `lookupFailed` and `CheckFirst` does not. Rollback is a revert.
