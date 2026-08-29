## Context

See proposal.md. Catalog, pointers, and `validateDatabaseSources` live in `plugin.go`. Empty geo pointer is a zero `dbsource.Config`; the BIN wrapper opens the bundled name. Missing pointer and bound URL without dir fail `New` today.

## Goals / Non-Goals

**Goals:**
- One inject + bind + validate path in `New` so Traefik overlay cannot drop the default row.
- Missing pointer and missing dir stay startable; type mismatch and unknown `databaseProvider` stay fatal.

**Non-Goals:**
- Programmatic ASN catalog (IP2Location or MaxMind).
- A City MMDB default.
- Embedding a live GeoLite file under `seeds/`.
- A new package for catalog defaults.

## Decisions

1. **Inject in `New`, not only `CreateConfig`.** Traefik overlays the operator map onto `CreateConfig`. A post-overlay insert keeps `default_ip2location` / `default_geolite` unless the operator already set that key.

2. **One LITE URL constant.** `pkg/ip2location` `DefaultLiteURL` in `names.go`. The old `liteDownloadURL` in `autoupdate.go` is gone from this tree.

3. **Temp dir is `filepath.Join(os.TempDir(), "traefik-geoblock")`.** A stable subdirectory survives plugin reload in the same process. README already warns that `/tmp` is not durable across container replace.

4. **Type check at pointer bind.** Compare the named row’s `databaseType` (after trim/lower) to the provider format. Empty row type is allowed (provider default). Unknown type on any catalog row stays `Normalize` fail.

5. **Missing pointer rewrite before open.** WARN, then treat as empty so IP2Location geo binds `default_ip2location`, MaxMind binds `default_geolite`, and IPinfo/ASN keep today’s empty-pointer Resolve.

6. **Unofficial Country GET, not an embedded GeoLite file.** Official GeoLite needs `accountId:licenseKey` and forbids redistributing a live MMDB. `default_geolite` uses `pkg/maxmind` `DefaultGeoliteURL` (P3TERX `download` branch Country file). Until a dated file exists, Resolve still opens the official dummy `GeoIP2-Country-Test.mmdb`. City and ASN stay operator-only. Operators who want the official permalink keep setting `databaseSources` themselves.

## Risks / Trade-offs

- [Temp dir wiped on recreate] → Mitigate: WARN; README already says mount a volume for durable dated files.
- [Operator names `default_ip2location` or `default_geolite` with a bad type] → Mitigate: keep their row; type check fails `New` if they also bind it to the wrong provider format.
- [Default URL GET in tests] → Mitigate: tests that only need seed set `path` on the operator row or isolate network; inject still runs but Resolve can open bundled BIN before GET succeeds.

## Migration Plan

Operators who already set `ip2location_source_geo` or `maxmind_source` and a catalog row are unchanged. Operators who set nothing get the reserved rows and a possible temp-dir WARN. Empty MaxMind now GETs unofficial Country instead of staying on the dummy seed only. Operators who pointed at a missing key no longer fail `New`. Operators who pointed a BIN provider at `mmdb` (or the reverse) start failing `New`.
