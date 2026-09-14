# Fix F-3 empty bypassHeaders value matches omitted headers

Fix F-3 from `geoblock-bug-hunt-2026-09-14.md` (F-3 section only).

F-3: a `bypassHeaders` entry with an empty value bypasses every request that omits the header. `blockSkipReason` compares `req.Header.Get(header)` with the configured value; absent header is `""`. YAML `X-Bypass:` with nothing after the colon, unset env substitution, or trimmed secret → every request `pass:bypass_header`. Prepare never rejects an empty bypass value. US client with `blockedCountries: [US]` and `defaultAllow: false` reaches the backend.

Files: `pkg/geoblock/plugin.go` `blockSkipReason`; `pkg/geoblock/config.go` Prepare never validates `BypassHeaders`.

Desired: empty bypass value cannot silently disable blocking. Fail loudly in Prepare for empty values, and/or require the header to be present (`Values` non-empty) before comparing. Shape to existing Prepare validation. Smallest durable delta. Document the reject-vs-ignore choice on explore.md if both are viable — unattended: assumed, pick the loud-fail in Prepare plus present-header check so a leftover empty map entry cannot open the gate at request time either.

Out of scope: F-1, F-2, F-4–F-9. Do not copy `zzz_proof_*` filenames; write proper product tests including `TestProofEmptyBypassHeaderValueBypassesEveryRequest` scenario (empty value must not yield 200/`pass:bypass_header` for a blocked country).

## F-3 from geoblock-bug-hunt-2026-09-14.md

- Severity: high
- Production impact: `blockSkipReason` compares `req.Header.Get(header)` with the configured value. An absent header reads as `""`, so a catalog entry whose value is empty — a YAML `X-Bypass:` with nothing after the colon, an unset environment substitution, a trimmed secret — matches every request that does not send the header. Blocking is silently disabled plugin-wide and the decision header says `pass:bypass_header`. Nothing in `Prepare` rejects an empty bypass value.
- Files: `pkg/geoblock/plugin.go:374-382`, `pkg/geoblock/config.go:179-222` (`Prepare` never validates `BypassHeaders`).
- Why it is real: the comparison has no "header present" test, and the empty string is the zero value of both sides. A US client with `blockedCountries: [US]` and `defaultAllow: false` reaches the backend.
- Evidence test: `pkg/geoblock/zzz_proof_policy_test.go` → `TestProofEmptyBypassHeaderValueBypassesEveryRequest`
- Suggested fix direction: require the header to be present (`req.Header.Values(header)` non-empty) before comparing, and have `Prepare` reject a `bypassHeaders` entry with an empty value so the misconfiguration fails loudly instead of opening the gate.
