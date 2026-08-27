## Context

See proposal.md for motivation. Today every helper is `package traefik_geoblock`. Yaegi evals `New` / `CreateConfig` on the module root package. Crowdsec-bouncer-traefik-plugin shows the root package may import `pkg/…` subpackages.

IPinfo Lite is a second DatabaseProvider. Full Lite MMDB download is token-only (`https://ipinfo.io/data/ipinfo_lite.mmdb?token=`). `github.com/ipinfo/go` is the HTTP API, not a local MMDB reader. Per-request lookup uses MMDB via vendored `oschwald/maxminddb-golang` v1 (Yaegi / go 1.21; not v2). Research: `knowledge/research/ext_ipinfo_lite-database/`.

## Goals / Non-Goals

**Goals:**
- Root package is only the Traefik middleware (config, ServeHTTP, policy).
- Helpers live under `pkg/` with one job each.
- Plugin depends on `dbprovider.Provider` for open, lookup, and lifecycle (including auto-update).
- Logger is stdout slog only.
- `databaseProvider: ipinfo` opens IPinfo Lite. Empty / `ip2location` stays IP2Location.
- Bundled `ipinfo_lite.mmdb` is the seed when the path is empty and the auto-update dir has no dated file.
- IPinfo auto-update is token-only; dated files in `ipinfo_databaseAutoUpdateDir`; hot-swap after download.

**Non-Goals:**
- MaxMind or any third vendor.
- Changing allow/block / CIDR / IP2Location token `file=` rules.
- Renaming the leftover `openspec/changes/fix-token-download-file-code` folder.
- Making `ipinfo` the default `databaseProvider`.

## Decisions

### Root + `pkg/` layout
Yaegi needs `New` and `CreateConfig` on the import path in `.traefik.yml`. Helpers move to:

| Import | Job |
| --- | --- |
| `pkg/dbprovider` | `Provider` interface, `Record`, enrich keys |
| `pkg/ip2location` | IP2Location BIN + optional ASN BIN, factory, wrapper, auto-update |
| `pkg/ipinfo` | IPinfo Lite MMDB + token auto-update |
| `pkg/dbutils` | BIN/MMDB filename date helpers |
| `pkg/fileutils` | exists / copy / search (`TRAEFIK_PLUGIN_GEOBLOCK_PATH`) |
| `pkg/iplookup` | radix CIDR helper + directory monitor |

Alternative: stay one package with files only. Rejected — caller asked for packages. Alternative: underscore names (`file_utils`). Rejected — Go package names.

### Vendor selector and prefixed settings
`databaseProvider` selects the implementation (`ip2location` default; `ipinfo` implemented; unknown fails). IP2Location-only Traefik keys are `ip2location_*`. IPinfo-only keys are `ipinfo_databaseFilePath`, `ipinfo_databaseAutoUpdate`, `ipinfo_databaseAutoUpdateDir`, `ipinfo_databaseAutoUpdateToken`. The plugin opens the provider through a switch; it does not type-assert a concrete wrapper. Request path never imports MMDB or IP2Location SDK types.

### Provider surface
`Lookup(ip string) (Record, error)` plus `Close()`. Auto-update starts inside the provider constructor when config says so. Plugin uses `Record.Country` for allow/block and `requestHeaderEnrich` for valued fields. `banIfError` is unchanged.

### IPinfo Lite mapping
Allow/block country is Lite `country_code` (ISO), not the country name. Enrich:

| Enrich key | Lite field |
| --- | --- |
| `country` | `country_code` |
| `country_name` | `country` |
| `continent` | `continent` |
| `continent_code` | `continent_code` |
| `isp` | `as_name` |
| `domain` | `as_domain` |
| `asn` | `asn` (`AS` prefix kept) |

Region/city are empty on the Record. `requestHeaderEnrich` still writes those headers as `null`. Do not fold Lite download into IP2Location `file=`.

### IPinfo auto-update
Same pattern as ASN LITE: dir required when auto-update is on (`New` fails without it). Token required to download; no token → error log, keep seed. Store `YYYYMMDD_ipinfo_lite.mmdb`. Daily ticker; hot-swap. Stale bundled snapshot is allowed.

### Yaegi and license
Vendor upstream `oschwald/maxminddb-golang` v1. After `go mod vendor`, run `scripts/apply-oschwald-yaegi-patch.ps1` (overlay in `patches/oschwald-maxminddb-golang/`). That swaps `Open` to `ReadFile`+`FromBytes` and deletes mmap files so Yaegi never imports `x/sys/unix` (`incomplete type ifreq`). Do not fork the decoder. README credits IPinfo (CC-BY-SA 4.0). Do not commit tokens.

### Logger
`createLogger(name, level, format)` → stdout text/json slog. Delete `bufferedFileWriter`. Drop `logBannedRequests` branch in `ServeHTTP`.

### Integration
Remove compose labels for deleted fields. Keep `geoblock-logheaders` with `logStatusDetailHeader` only. Drop remediaiton-header assertions (`X-Geoblock-Action`). Drop `X-Geoblock-Status` asserts. Add `/ipinfo` compose + Pester for Lite enrich headers. Package tests are enough to propose; `Test-Integration.ps1` is a before-merge check.

## Risks / Trade-offs

- [Yaegi import of local `pkg/`] → Mitigation: same pattern as crowdsec; integration compose already mounts the repo as a local plugin.
- [Yaegi cannot fetch modules] → Mitigation: commit `vendor/` including oschwald v1 (not v2).
- [Operators still set removed keys] → Traefik will ignore unknown labels or fail decode depending on version; README states the break.
- [Bundled MMDB goes stale] → Mitigation: token auto-update; stale seed still opens.
- [ASN string format differs from IP2Location] → Keep IPinfo `AS` prefix; do not normalize.
- [Tests import moved types] → Update imports; keep behavior tests.

## Migration Plan

1. Deploy plugin that lacks the removed fields.
2. Set `logStatusDetailHeader` and keep that name in access logs.
3. Remove file log mounts used only for `logPath`.
4. To use IPinfo: set `databaseProvider: ipinfo`. Optional token + auto-update dir. Empty path uses the bundled MMDB.
5. Rollback: previous plugin version still accepts the old keys and has no `ipinfo` selector.

## Open Questions

None. Explore decisions are in `devstate/explore.md`.
