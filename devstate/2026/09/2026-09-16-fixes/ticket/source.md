# Rescue proof-test findings as product contracts

Chat-originated. Human confirmed `/opd-workflow` on branch `20260916fixes` / current workspace (no worktree). Decisions on the failing proof cases:

1. CIDR allow `/0` vs block `/32`: the more specific prefix wins (block).
2. Empty `bypassHeaders` value: already fixed in https://github.com/david-garcia-garcia/traefik-geoblock/commit/579856d2819869ced7651e63eede9d15b34c2fa0 — do not re-implement; the proof is obsolete.
3. `countryHeader` also mapped to a non-country enrich key: keep current behavior (enrich wins). At load, warn that the country header is being filled with a non-country value.
4. `mode=block` must work without catalog/enrich. `countryHeader` is mandatory in block mode (otherwise there is no header to read). If that header is missing on the request, apply the same logic as a lookup error (`banIfError`).
5. A request with no client IP is impossible: treat as error — block depending on `banIfError`, and log a warning.
6. A corrupt dated catalog file must not prevent startup: fall back to the configured seed and warn. Same for BIN and MMDB, including the Traefik `New` path.

Also: turn the leftover `zzz_proof_*` probes into real tests (or delete them when already covered). Do not keep always-fatal proof files.
