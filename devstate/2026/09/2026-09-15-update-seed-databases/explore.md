# Explore
IssueKey: 2026-09-15-update-seed-databases

## Concepts

**Bundled seed** — the committed file under `seeds/` named by a reserved catalog `defaultFile`. Resolve opens it when there is no dated Latest and no operator `path` (`knowledge/devdocs/core_geoblock_database_source.md`).

**Seed CLI** — `tools/dbdownload` GETs the free IP2Location LITE ZIP, extracts `IP2LOCATION-LITE-DB1.IPV6.BIN`, and writes only if `1.1.1.1` `country_short` is `US`. `plugin.go` `go:generate` points that CLI at `seeds/`.

**Official dummy** — MaxMind `GeoIP2-Country-Test.mmdb` on `maxmind/MaxMind-DB` `test-data`. Not a live GeoLite edition (`knowledge/research/ext_maxmind_geolite2-database/notes.md`).

**IPinfo full Lite** — token-gated `https://ipinfo.io/data/ipinfo_lite.mmdb`. The 100-row sample is a different edition (`knowledge/research/ext_ipinfo_lite-database/notes.md`).

```
Lookup, empty path
        │
        ▼
 dated Latest? ──yes──► open dated file
        │ no
        ▼
 operator path file? ──yes──► open that path
        │ no
        ▼
 seeds/<defaultFile>
```

## Decisions

- Refresh is a same-name byte replace. No new vendor, catalog key, or seed filename.
- Measured 2026-09-15 against official sources (temp dir, not the worktree):
  - IP2Location LITE ZIP extracted BIN is **newer**: 11,702,411 bytes, SHA256 `D23BCFAAAC2FD18135F954D2D3223C8542B65D6B0DDFD99912F18BB00CCFB1BE`. Committed seed is 10,349,586 bytes, SHA256 `70C6EE0DC7C9E21726DB50C435ABBC355B3122C9D03838F873B456A86D7A7522`.
  - Probe (ip2location-go/v9): new BIN and committed BIN both return `1.1.1.1` → `AU` and `8.8.8.8` → `US`. `go run ./tools/dbdownload` failed verify on the new extract (`1.1.1.1` is not `US`). The same sentinel would fail the committed seed.
  - MaxMind dummy from `maxmind/MaxMind-DB` `main` `test-data/GeoIP2-Country-Test.mmdb` is **byte-identical** to the committed file (SHA256 `B37601903448683D241AF52893C8CBF0FED461E0CDEBE0BFACA01891FDEB6DB9`, 19,492 bytes).
  - No `IPINFO_TOKEN` / `IPINFO_LITE_TOKEN` / `IPINFO_ACCESS_TOKEN` in the environment. Official full Lite was not downloaded.
- Implement writes the official LITE BIN into `seeds/IP2LOCATION-LITE-DB1.IPV6.BIN` (copy of the verified extract, not via the CLI write path). Leave MaxMind and IPinfo bytes unchanged.
- Do not change `tools/dbdownload` verify (requirement Out of scope: name did not move). Pre-existing sentinel mismatch is a follow-up.
- Do not replace IPinfo with the 100-row sample.
- Usage packets already cover Resolve and reserved rows. No Language write. No new research folder (existing `ext_ipinfo_lite-database` and `ext_maxmind_geolite2-database` were enough; LITE ZIP URL is owned by `tools/dbdownload`).
- This change does not set or reconstruct client address; lookup still uses the existing wrapper.

## Open questions

- Q: How do we refresh `seeds/ipinfo_lite.mmdb` without an IPinfo account token?
  Rank: additive incidental — no new contract or caller migration; Desired names that file, but official full Lite is token-gated and Out of scope forbids new IPinfo download automation
  Decision: assumed — leave the committed IPinfo snapshot; do not substitute the 100-row public sample (wrong edition).
  By: explore

- Q: Should `tools/dbdownload` stop requiring `1.1.1.1` → `US`?
  Rank: bounded incidental — two existing call sites enumerated (`tools/dbdownload/main.go` `verifyDatabase`, `plugin.go` `go:generate`); Out of scope unless the LITE ZIP or BIN name moved (neither did)
  Decision: assumed — do not change verify this run; copy official LITE bytes into `seeds/`; record the pre-existing sentinel as a follow-up (committed LITE also returns `AU`).
  By: explore

- Q: Is `GeoIP2-Country-Test.mmdb` on `maxmind/MaxMind-DB` `test-data` newer than the committed 19,492-byte file?
  Rank: additive asked — Desired names that seed; criterion is a newer official build of the same fixture
  Decision: resolved — SHA256 matches `maxmind/MaxMind-DB` `main` `test-data/GeoIP2-Country-Test.mmdb`; no byte replace.
  By: explore

- Q: Does a newer IP2Location LITE still map `1.1.1.1` → `US` so `tools/dbdownload` verify succeeds?
  Rank: additive asked — Unknowns on requirement.md; Desired is the newer official LITE
  Decision: resolved — official extract and committed seed both return `AU`; verify already fails; `8.8.8.8` stays `US`. Policy tests already treat `1.1.1.1` as AU.
  By: explore

- Q: Will a newer IPinfo Lite still match pinned org/ASN rows in `pkg/dbwrappers/ipinfo_test.go`?
  Rank: additive asked — Unknowns on requirement.md; Desired names that seed
  Decision: assumed — IPinfo bytes are not refreshed this run, so those pins stay against the committed snapshot.
  By: explore
