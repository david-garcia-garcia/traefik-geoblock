## Context

See proposal.md — Why. Traefik calls `CreateConfig` then mapstructure-decodes the operator map onto that value. An omitted `mode` key leaves the `CreateConfig` field. Today that field is `""` and `NormalizeMode` maps empty/whitespace to `disabled`.

## Goals / Non-Goals

**Goals:**
- Omitted and empty `mode` become `enrichandblock` at both overlay (`CreateConfig`) and normalize (`NormalizeMode`).
- Explicit `mode: disabled` stays pass-through.

**Non-Goals:**
- Restoring Config `enabled`.
- Changing other `CreateConfig` defaults.
- Changing catalog insert rules for lookup modes.

## Decisions

- **Both `CreateConfig` and `NormalizeMode` change.** Overlay keeps `CreateConfig.Mode` when the key is omitted. `NormalizeMode("")` covers `&Config{}` and whitespace. Alternative: only `NormalizeMode` — Traefik would still work after `Prepare`, but `CreateConfig` would document the wrong default and any reader of the raw struct before `Prepare` would see `""`.
- **Keep the four mode values.** Alternative: drop `disabled` and treat only omit as off — rejected; operators need an explicit off switch.

## Risks / Trade-offs

- [Tests that construct `&Config{}` without `Mode` now enter lookup] → Set `Mode` explicitly in those tests, or give them a valid lookup config (`IPHeaders`, catalog). Prefer explicit `Mode` on non-default cases.
- [Operators who omitted `mode` after the `plugin-request-mode` break expected pass-through] → Ticket prefers pre-`mode` compatibility (`enabled: true` ≈ `enrichandblock`). Document the default in README.

## Migration Plan

Ship the default. Operators who want pass-through set `mode: disabled`. Rollback is revert.
