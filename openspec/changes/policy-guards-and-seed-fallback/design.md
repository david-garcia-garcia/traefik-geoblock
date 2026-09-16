## Context

See proposal.md. Dest `decide` requires `allowedNetworkLength > 0`, so a `/0` allow is treated as no match and wins in the `else` branch. `blockFromHeader` already uses `banIfError` for an empty country header; an empty `GetRemoteIPs` slice never hits that path. `Resolve` returns Latest with no open; `lifecycle.initialize` publishes it and fails. `Lookup` on `mode=block` calls `p.db.Lookup` while `bindDatabase` left `p.db` nil.

## Goals / Non-Goals

**Goals:**

- Longer CIDR prefix wins, including length 0.
- Empty hop list is an error owned by `GetRemoteIPs` (reuse the slice; do not re-parse).
- Unreadable dated Latest → warn + seed; New succeeds.
- Warn when `countryHeader` enrich key is not `country`.
- Block-mode `Lookup`/`CheckAllowed` error, no panic.
- Ordinary tests; delete `zzz_proof_*`.

**Non-Goals:**

- Re-doing empty `bypassHeaders` reject.
- Changing enrich-wins mapping.
- Deleting or quarantining the dated file.
- Opening catalog sources in `mode=block`.
- Rejecting omitted `countryHeader` (default `X-IPCountry` stays).

## Decisions

- Compare prefix lengths with `>= 0` matches (`found` from `Contains`), not `> 0`. If both found and block length > allow length, block. Else if allow found, allow. Else if block found, block.
- Empty hop list: before the hop loop, if `len(remoteIPs)==0`, warn and apply `banIfError` the same way as empty country.
- Seed fallback lives in `lifecycle.initialize` / `publish`: if the chosen target fails to open, try catalog `Path` then `BundledFile`. Do not put open/validate inside `Resolve` (path picker stays path picker).
- Warning for non-country `countryHeader` mapping: one `logger.Warn` in `Prepare` after `foldCountryHeader` (or immediately after normalize), naming the header and the mapped key.
- `Lookup`: if `p.db == nil`, return `fmt.Errorf("...")` that the catalog is not bound. `CheckAllowed` already goes through `recordForLookup` → `Lookup`.

## Risks / Trade-offs

- [A `/0` allow plus a more-specific block now blocks] → That is the documented contract. Operators who used `/0` as a total allow must drop the specific block.
- [Corrupt dated file stays on disk and keep-current retries] → Repeated warn/error logs until a good dated file appears. Bound the ask: do not delete operator files.
- [Block-mode `Lookup` now errors] → Any caller that ignored the panic will see an error; Traefik `ServeHTTP` does not call it.

## Migration Plan

- Deploy the new plugin binary. No config key change.
- Operators with empty `countryHeader` keep `X-IPCountry`.
- Operators with a city mapping on `countryHeader` see a new warning only.

## Open Questions

None — rows live on `devstate/explore.md`.
