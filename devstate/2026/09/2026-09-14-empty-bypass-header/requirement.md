# Requirement
IssueKey: 2026-09-14-empty-bypass-header

## Problem
A `bypassHeaders` map entry whose value is empty matches every request that omits that header. `blockSkipReason` treats an absent header as `""`, so YAML `X-Bypass:` with nothing after the colon, an unset env substitution, or a trimmed secret skips blocking for all traffic. Decision header is `pass:bypass_header`. A US client with `blockedCountries: [US]` and `defaultAllow: false` reaches the backend. `Prepare` never rejects the empty value.

## Current (code)
- `pkg/geoblock/plugin.go` `blockSkipReason` (374–382): for each `p.bypassHeaders` pair, compares `req.Header.Get(header)` to `expectedValue`. `Get` returns `""` when the header is absent. Equal → `PhaseBypassHeader` (`bypass_header`).
- `pkg/geoblock/plugin.go` `ServeHTTP` (332–336): non-`none` skip reason writes `pass:{reason}` via `setDecisionLogHeader` (304–307) and calls `next` (backend) without country/CIDR checks.
- `pkg/geoblock/plugin.go` `NewCore` (197): copies `cfg.BypassHeaders` onto the plugin with no value check.
- `plugin.go` `New` (68–70): `geoblock.Prepare` is the Traefik constructor gate; a config that `Prepare` accepts is applied.
- `pkg/geoblock/config.go` `Prepare` (179–222): validates mode, `DisallowedStatusCode`, non-empty `IPHeaders` (192–194), `IPHeaderStrategy`, `CountryHeader`, catalog. Does not read `BypassHeaders`.
- `pkg/geoblock/config.go` `BypassHeaders` (97–99): map of header name → value; comment says match skips geoblocking; no non-empty invariant.
- `pkg/geoblock/plugin_policy_test.go` `BypassHeaders` (213–291): non-empty secrets; omitted header with a non-empty configured value is 403. No empty-value case.
- `pkg/geoblock/plugin_observe_test.go` `BypassHeader_should_set_pass_bypass_header` (227–265): matching non-empty `X-Bypass` yields `pass:bypass_header`. Next handler is teapot (418), not 200.
- `pkg/geoblock/plugin_config_test.go` `EmptyIPHeaders` (698–711): empty `IPHeaders` fails `newRoute`/`Prepare`. No sibling test for empty bypass values.
- `README.md` (379–380, 475): documents `bypassHeaders` with a non-empty example; does not forbid empty values.
- `pkg/geoblock/zzz_proof_policy_test.go` — not found on `origin/master`.

## Desired
- Empty bypass value cannot silently disable blocking.
- Fail loudly in `Prepare` for empty bypass values, shaped like existing `Prepare` validation (`IPHeaders` empty-reject).
- Also require the header to be present (`req.Header.Values` non-empty) before comparing, so a leftover empty map entry cannot open the gate at request time.
- Smallest durable delta.
- Product tests covering `TestProofEmptyBypassHeaderValueBypassesEveryRequest` scenario: empty value must not yield pass-through / `pass:bypass_header` for a blocked country. Do not copy `zzz_proof_*` filenames.

## Affected
- `pkg/geoblock/config.go` `Prepare`
- `pkg/geoblock/plugin.go` `blockSkipReason`
- Product tests under `pkg/geoblock/` (`plugin_config_test.go` and/or `plugin_policy_test.go`)
- `README.md` bypassHeaders contract if operator-facing reject is documented

## Out of scope
- F-1, F-2, F-4–F-9
- Copying `zzz_proof_*` filenames
- Changing match behavior for non-empty bypass values
- Other skip reasons (`ignoreVerbs`, path regex)

## Unknowns
- Whether Traefik/Yaegi YAML `X-Bypass:` (empty after colon) always materializes as `""` in the map versus omitting the key (code path is `Get` vs `""`; YAML decode not shown in this tree).
- Whether whitespace-only values should be treated as empty (ticket names a trimmed secret).
- Whether an empty header *name* with a non-empty value is in scope (ticket is empty *value*).

## Tensions
- Ticket offers Prepare-reject and/or present-header check; caller unattended assumed both (loud-fail plus present-header). Not a product extra — recorded as the proceed choice.
- Ticket says empty value must not yield `200` / `pass:bypass_header`; dest tests use a teapot next handler, so pass-through is 418, not 200. Scenario is “must not skip blocking,” not a literal 200.
- Evidence file `zzz_proof_policy_test.go` is cited in the dump and is not on dest; implement must write a proper product test, not that filename.
