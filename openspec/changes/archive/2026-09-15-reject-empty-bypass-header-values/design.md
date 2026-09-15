## Context

See proposal.md — Why. `Prepare` already rejects `len(IPHeaders) == 0` after the `ModeDisabled` early return and does not read `BypassHeaders`. `blockSkipReason` compares `req.Header.Get(header)` to the configured value; `Get` is `""` when the header is absent. `net/http.Header` owns presence (`Values`) and the first value (`Get`). `newTestPlugin` / `New` both call `Prepare` then `NewCore`; `NewCore` assigns `cfg.BypassHeaders` with no copy.

## Goals / Non-Goals

**Goals:**
- Empty and whitespace-only `bypassHeaders` values fail `Prepare` when mode is not `disabled`.
- An absent request header cannot match a leftover empty map entry.
- Existing non-empty bypass matches stay byte-equal `Get` compares after a presence check.

**Non-Goals:**
- Empty header *names*.
- Skipping empty `expectedValue` at request time (Prepare owns empty config).
- Validating `BypassHeaders` when `mode` is `disabled`.
- YAML/env decode of `X-Bypass:`; the Go map after Traefik decode is the contract.
- Other skip reasons (`ignoreVerbs`, path regex).
- README processing-order mismatch (bypass listed before ignore verbs; code is the reverse).
- Usage-packet writes (`BypassHeaders` / `PhaseBypassHeader` already named; `skipBlock` already shown).

## Decisions

- **Both layers.** `Prepare` rejects empty values so Traefik/`New` cannot load the misconfiguration. `blockSkipReason` requires `len(req.Header.Values(header)) > 0` before the existing `Get` compare so a leftover empty entry cannot match an absent header. Alternative: reject-only — rejected; `NewCore` without `Prepare` would still open the gate. Alternative: ignore empty entries at request time and still load — rejected; that hides the misconfiguration. Alternative: skip empty `expectedValue` in the loop as a third layer — rejected; Prepare owns empty config.
- **Empty is `strings.TrimSpace(value) == ""`.** Same empty test as `CountryHeader`. Ticket names a trimmed secret as a producer of `""`. Do not trim at request time (would change non-empty match).
- **Presence from `Header.Values`, then `Get` for the value.** Do not treat `Get == ""` as absent (a present empty header is present). Do not re-derive presence from another signal.
- **Leftover policy test mutates the live map.** `newRoute` cannot load an empty value once Prepare rejects. Same-package test: `newTestPlugin` with a valid config, then `plugin.bypassHeaders = map[string]string{"X-Bypass": ""}`, `ForRoute`, ServeHTTP without that header. Alternative: call `NewCore` without `Prepare` — rejected; that also skips catalog bind and is a larger fixture.
- **Fold into `core_geoblock_plugin_request-mode`.** Bugfix plus two invariants on the existing plugin-creation and skip-block path. A new `bypass-headers` leaf would name the whole skip token while this change only adds the empty-value contract.

## Risks / Trade-offs

- [Operators who already ship `X-Bypass:` with an empty value] → plugin creation fails until they set a real secret or remove the key. That is the fix.
- [Present empty header + leftover empty expected] → `Values` is non-empty, so the compare can still match. Presence semantics; Prepare is the loud fail for that config.
- [Whitespace-only values] → fail at load; they would not have matched an absent header via `Get` anyway, but they fail the leftover-secret class at creation.

## Migration Plan

Ship. Operators with an empty bypass value must put a non-empty secret in the map or delete the entry. Rollback is revert.
