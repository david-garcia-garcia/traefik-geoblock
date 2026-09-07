## Why

`blockFromHeader` gates the deny on `passReason == PhaseNone` and sets `passReason` on the first **allowed** hop, so the first allowed hop discards every later denial. Under `ipHeaderStrategy: CheckAll`, `blockedIPBlocks` is then unenforceable on any hop but the first — with `blockedIPBlocks: ["1.1.1.0/24"]` and `defaultAllow: true`, `X-Forwarded-For: 1.1.1.1` is blocked but `8.8.8.8, 1.1.1.1` passes — and a country verdict is discarded whenever an earlier hop was allowed, typically a private one.

That contradicts this capability's own requirement, "`CheckAll` SHALL still apply CIDR and private per selected IP", and the README sentence pointing operators at CIDR lists for per-hop control. It also makes `CheckAll` produce the same blocking decision as `CheckFirst` for any chain of well-formed addresses, since a later hop can only deny and the deny is gated shut once hop 0 is allowed.

## What Changes

- **BREAKING (behavior):** any selected hop may deny. The deny and the in-loop `banIfError` ban no longer require `passReason == PhaseNone`. `passReason` keeps one job: naming the first allowing phase for the `pass:{reason}` decision header.
- No config change and no new option. For chains of well-formed addresses, `ipHeaderStrategy: CheckFirst` reproduces the previous verdict.
- README documents the behaviour change, and that `allowedIPBlocks` cannot allow a private hop because `decide` answers private hops from `allowPrivate` first.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_plugin_request-mode`: the block stage's per-hop guarantee is stated as enforceable — a hop after the first can deny — and `passReason` is confined to the pass log reason.

## Impact

- `pkg/geoblock/plugin.go` (`blockFromHeader`)
- `pkg/geoblock/plugin_ipheaders_test.go` (`TestIPHeaderStrategy_CheckAllDeniesOnAnyHop`)
- `README.md`, `knowledge/devdocs/core_geoblock_plugin_request-mode.md`
- **Operators:** `CreateConfig` sets `CheckAll` and leaves `allowPrivate` false, so a chain whose selected hops include a private address changes from pass to `403 block:allow_private`. Remedy is `allowPrivate: true` or `ipHeaderStrategy: CheckFirstNonePrivate`.
