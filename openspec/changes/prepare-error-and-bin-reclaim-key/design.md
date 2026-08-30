## Context

See proposal.md — Why. Traefik logs the `New` error string as `ERR error="<middleware>@<provider>: <message>"` once per router. `reclaim_put` logs the process-table key, not the wrapper logger’s catalog `key` attr.

## Goals / Non-Goals

**Goals:**
- One sentence on the missing-fields error that an operator can act on (middleware never attached).
- Reclaim table keys for wrappers name the catalog row and keep the hash so share/dispose identity is unchanged.

**Non-Goals:**
- Defaulting a preset when fields are missing.
- Changing plugin-instance keys (`plugin:<name>:<hash>`).
- Changing Traefik’s per-router `New` or how it prints `ERR`.

## Decisions

- **Implication suffix, not a rewrite of the cause.** Keep `set fields or fieldsPreconfigured` so existing tests and grep still find the cause. Append `; plugin does not start; this middleware is not applied`. Alternative: a long essay — rejected (Traefik already repeats the line per router).
- **Catalog key then hash.** `binKey` / `mmdbKey` become `prefix + Source.Key + ":" + configHash(cfg)`. Hash still covers the full config (including `Source.Key`), so two rows with different names stay distinct. Alternative: only the catalog key — rejected (two different files under the same name would collide). Alternative: extra slog attr on reclaim — rejected (ticket asked for the key itself).
- **MMDB same shape.** Ticket quoted `bin:`. Same table and diagnosis gap; apply the same prefix rule. Alternative: BIN only — rejected (assumed in explore.md).

## Risks / Trade-offs

- [Operators grep `bin:<hex>` only] → Mitigation: prefix stays `bin:`; hex is still the last segment.
- [Empty `Source.Key`] → Mitigation: emit `bin::<hash>`; production catalog always has a map key.

## Migration Plan

Ship in the next plugin build. No config migration. After upgrade, `reclaim_put` keys gain a middle segment; dashboards that exact-match old keys need a prefix match.
