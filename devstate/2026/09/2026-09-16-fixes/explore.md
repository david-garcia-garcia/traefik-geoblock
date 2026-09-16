# Explore

Reproduced on `20260916fixes` working tree (throwaway `zzz_proof_*`, not product tests). Dest behavior is the same code paths. No product fix in this phase.

Devdocs consumed: `knowledge/devdocs/index.md` → request-mode, wrapper, source. Language already names Mode, Country header, BypassHeaders, Wrapper, Lifecycle, Source, Resolve order. Usage already says missing `countryHeader` uses `banIfError`, and Resolve prefers dated Latest with no validate. No packet produced this phase.

Research: no third-party write. CIDR compare, Prepare warning, seed fallback, and empty-IP error are this repo’s contracts. Client hops stay on `GetRemoteIPs`.

```
Resolve Latest (no validate) ──► publish dated ──► fail open ──► New dies
                                      │
                                      ▼ desired
                                 warn + publish Path/BundledFile

decide: allow /0 (len 0) ──► else ──► allowed wins
        desired: compare prefix lengths including 0; /32 block wins

GetRemoteIPs [] ──► blockFromHeader loop ──► pass:none
             desired: error + banIfError + warn
```

## Concepts

- `pkg/geoblock/plugin.go` `decide` — CIDR specificity uses `allowedNetworkLength > 0`. `/0` is a real match with length 0. README already says more specific wins.
- `pkg/geoblock/plugin.go` `blockFromHeader` — empty/`null` country already follows `banIfError`. Empty `remoteIPs` never enters that error path; it writes `pass:none`.
- `pkg/geoblock/ip.go` `GetRemoteIPs` — owner of the hop list. Empty means that owner found no hop. Do not re-parse `RemoteAddr` here.
- `pkg/geoblock/plugin.go` `Lookup` / `bindDatabase` — block mode leaves `p.db` nil. Exported `Lookup` panics. `ServeHTTP` does not call it.
- `pkg/geoblock/config.go` `Prepare` — empty `CountryHeader` becomes `X-IPCountry` for every mode. Empty bypass values already rejected (`579856d`). `foldCountryHeader` lets an existing enrich mapping win; no warning.
- `pkg/dbsource/resolve.go` `Resolve` — dated Latest wins with no open. `lifecycle.initialize` publishes that path; BIN `seedFirst` only uses `BundledFile`, not catalog `Path`.
- Request-mode usage already documents missing country → `banIfError` and enrich-before-block. Wrapper usage does not say “unreadable dated file falls back to seed”.

## Decisions

- Fix `decide` so prefix length 0 is a match. Compare allow vs block lengths; longer prefix wins. `/32` block beats `/0` allow.
- Do not touch empty-bypass product code. Delete the obsolete proof (config tests already reject empty values).
- Keep enrich-wins-over-`countryHeader`. Warn at `Prepare` / `NewCore` when that mapping’s key is not `country`.
- Block mode stays catalog-free. Keep the `X-IPCountry` default so omitted `countryHeader` still names a header. Missing header on the request already uses `banIfError`; keep that. `Lookup`/`CheckAllowed` on a block-mode plugin return an error, no panic.
- Empty `GetRemoteIPs` is a lookup-class error: warn, then `banIfError`. Do not invent a hop.
- When publish of Latest fails, fall back to catalog `Path` then `BundledFile`, warn, start. Keep-current may retry Latest; a failed hot-swap stays on the seed (existing error log). BIN and MMDB share `lifecycle.initialize`.
- Replace `zzz_proof_*` with ordinary tests (or delete when covered). No `zzz_proof_*` names in product tests.

## Open questions

- Q: Who owns the client hop list when it is empty?
  Rank: bounded asked — requirement Desired “no extractable client IP”; 1 owner (`GetRemoteIPs` in `pkg/geoblock/ip.go`; searched `**/pkg/geoblock/*.go` for `GetRemoteIPs` / `RemoteAddr` parse)
  Decision: resolved — reuse `GetRemoteIPs` output. Empty slice is “no hop”. `blockFromHeader` must not re-parse `RemoteAddr` or headers.
  By: explore

- Q: In `mode=block`, does mandatory `countryHeader` reject an omitted field, or does the `X-IPCountry` default still apply?
  Rank: bounded asked — requirement Desired “countryHeader is mandatory in block mode”; 1 writer (`Prepare` in `pkg/geoblock/config.go`); callers go through `Prepare` (`New`, `newTestPlugin`, config tests)
  Decision: assumed — keep the default. The job is “block mode always has a header name to read.” Rejecting omit would fail configs that rely on `X-IPCountry`. Warn-only if we later want operators to set it explicitly.
  By: explore

- Q: After a corrupt dated file is skipped, should keep-current keep selecting that same Latest?
  Rank: additive asked — requirement Desired “keep-current may retry later”
  Decision: assumed — yes. Leave Latest on disk. Failed hot-swap logs and keeps the seed. Do not delete or rename the dated file in this change.
  By: explore

- Q: Should exported `Lookup` / `CheckAllowed` on a block-mode plugin return an error, or is ServeHTTP enough?
  Rank: additive asked — requirement Unknowns; Desired “mode=block works without catalog”
  Decision: assumed — return `fmt.Errorf` that the catalog is not bound. ServeHTTP stays on `countryHeader`. Do not open sources in block mode.
  By: explore
