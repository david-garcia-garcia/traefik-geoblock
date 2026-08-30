## Why

Operators who omit Config `mode` currently get pass-through (`disabled`). Existing installs that never set `mode` must keep GeoIP lookup and country allow/block. Empty and omitted `mode` must be `enrichandblock`.

## What Changes

- Empty or omitted `mode` (including `CreateConfig` before overlay) becomes `enrichandblock`.
- Explicit `mode: disabled` stays pass-through.
- Unknown `mode` still fails plugin creation.
- README and the request-mode usage packet follow the same default.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_plugin_request-mode`: Empty `mode` SHALL be `enrichandblock` (was `disabled`).

## Impact

- `pkg/geoblock/config.go` — `CreateConfig`, `NormalizeMode`.
- Tests for empty / `CreateConfig` mode (`pkg/geoblock/plugin_mode_test.go`, `plugin_config_test.go`).
- `README.md` empty-mode sentence.
- `knowledge/devdocs/core_geoblock_plugin_request-mode.md` Language and How to use.
