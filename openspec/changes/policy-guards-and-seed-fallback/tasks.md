## 1. Policy

- [x] 1.1 `decide`: longer CIDR prefix wins, including length 0. Test: `/0` allow + `/32` block on `8.8.8.8` is blocked. No `zzz_proof_*`.
- [x] 1.2 Empty `GetRemoteIPs`: warn + `banIfError`. Do not re-parse. Test both `banIfError` true and false.
- [x] 1.3 `Prepare` warns when `countryHeader` enrich key is not `country`. Enrich still wins. Test load + warning.
- [x] 1.4 `Lookup`/`CheckAllowed` on `mode=block` return catalog-not-bound error. Test no panic.

## 2. Seed fallback

- [x] 2.1 `lifecycle.initialize`: if opening Latest fails, warn and publish catalog `path` then `BundledFile`. BIN and MMDB. Do not delete the dated file.
- [x] 2.2 Tests: corrupt dated BIN and MMDB start on the seed; Traefik `New` succeeds. No `zzz_proof_*`.

## 3. Cleanup

- [x] 3.1 Delete leftover `zzz_proof_*` (dbwrappers, geoblock, iplookup, repo root).
- [x] 3.2 README / request-mode usage only if the operator-facing warn or seed fallback is missing there.
