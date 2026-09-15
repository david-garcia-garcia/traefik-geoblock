## Why

A `bypassHeaders` map entry whose value is empty (YAML `X-Bypass:` with nothing after the colon, an unset env substitution, or a trimmed secret) matches every request that omits that header. `blockSkipReason` compares `req.Header.Get` to the configured value, and `Get` returns `""` when the header is absent, so a US client under `blockedCountries: [US]` and `defaultAllow: false` reaches the backend with `pass:bypass_header`. `Prepare` never rejects the empty value.

## What Changes

- **BREAKING (misconfigured load):** `Prepare` rejects a `bypassHeaders` value that is empty after `TrimSpace`, in the same constructor gate as empty `IPHeaders`. `mode: disabled` still returns before that check.
- `blockSkipReason` requires the request header to be present (`req.Header.Values` non-empty) before comparing to the configured value, so a leftover empty map entry cannot match an absent header.
- Non-empty bypass matches stay the same.
- Package tests: `Prepare` reject (sibling of `EmptyIPHeaders`) and a leftover empty map entry must not skip blocking / must not write `pass:bypass_header` for a blocked-country request. No `zzz_proof_*` filenames.
- README: one sentence next to the `bypassHeaders` example that empty values are rejected at plugin creation.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_plugin_request-mode`: empty `bypassHeaders` values cannot skip the block stage. Plugin creation fails them; the skip path requires the header present before compare.

## Impact

- `pkg/geoblock/config.go` `Prepare`
- `pkg/geoblock/plugin.go` `blockSkipReason`
- `pkg/geoblock/plugin_config_test.go`, `pkg/geoblock/plugin_policy_test.go`
- `README.md` (`bypassHeaders` example)
- **Operators:** a Traefik config that already ships an empty bypass value will fail plugin creation instead of opening the gate. Non-empty secrets are unchanged.
