## Why

[PascalMinder/geoblock#67](https://github.com/PascalMinder/geoblock/issues/67) reports CrowdSec `crowdsecurity/traefik-logs` failing `UnmarshalJSON` on GeoBlock lines shaped `INFO: GeoBlock: …`. This tree already emits slog text (`time=…`) or slog JSON — that prefix is gone — but nothing asserts the line shapes CrowdSec’s current hub parser skips or decodes. Without that lock, a logger change can restore the filed prefix or emit a `{`-prefixed invalid JSON line and the warning returns silently.

## What Changes

- Package tests that capture stdout from `New`, `NewBootstrap`, `NewOwner`, and `PluginLogger`/`CreateConfig` and assert: no `INFO: GeoBlock:` prefix; default/text lines do not start with `{`; `logFormat: json` lines `json.Unmarshal` as objects.
- Spec scenarios on the existing observability leaf for those line-shape invariants.
- No default `logFormat` flip. No `NewBootstrap`/`NewOwner` format parameter (debt).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_observability_decision-header`: stdout slog lines SHALL NOT use the #67 `INFO: GeoBlock:` prefix; text format SHALL NOT start with `{`; json format SHALL be a JSON object per line.

## Impact

- `pkg/logging/logging_test.go` — line-shape asserts.
- `pkg/geoblock` CreateConfig / PluginLogger tests — default format + logger bytes.
- CrowdSec Hub is not changed. Operators keep `logFormat: text` (default) or `json`.
