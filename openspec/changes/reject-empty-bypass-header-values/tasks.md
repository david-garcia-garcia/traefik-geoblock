## 1. Prepare reject

- [ ] 1.1 After the `ModeDisabled` early return, reject any `BypassHeaders` value whose `TrimSpace` is empty; leave an empty map valid; do not reject empty header names
- [ ] 1.2 Note on the `BypassHeaders` field comment that values cannot be empty
- [ ] 1.3 `plugin_config_test.go` sibling of `EmptyIPHeaders`: `""` and whitespace-only fail `newRoute`; empty map still succeeds; `mode: disabled` with `""` still succeeds

## 2. Present-header skip

- [ ] 2.1 In `blockSkipReason`, require `req.Header.Values(header)` non-empty before the existing `Get` compare; do not skip empty `expectedValue` in the loop
- [ ] 2.2 `plugin_policy_test.go`: after `newTestPlugin` with a valid config, set `bypassHeaders` to `{"X-Bypass": ""}`; omitted header on a US client (`blockedCountries: [US]`, `defaultAllow: false`) is blocked and is not `pass:bypass_header` (set `logStatusDetailHeader`). No `zzz_proof_*` filename
- [ ] 2.3 Keep the existing non-empty `BypassHeaders` policy cases

## 3. Operator docs

- [ ] 3.1 One sentence next to the README `bypassHeaders` example that empty values are rejected at plugin creation, matching the `ipHeaders` empty-reject line. Do not rewrite processing order
