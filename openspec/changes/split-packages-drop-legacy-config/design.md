## Context

See proposal.md for motivation. Today every helper is `package traefik_geoblock`. Yaegi evals `New` / `CreateConfig` on the module root package. Crowdsec-bouncer-traefik-plugin shows the root package may import `pkg/…` subpackages.

## Goals / Non-Goals

**Goals:**
- Root package is only the Traefik middleware (config, ServeHTTP, policy).
- Helpers live under `pkg/` with one job each.
- Plugin depends on `dbprovider.Provider` for open, lookup, and lifecycle (including auto-update).
- Logger is stdout slog only.

**Non-Goals:**
- MaxMind or any second provider implementation.
- Changing allow/block / CIDR / token `file=` rules.
- Renaming the leftover `openspec/changes/fix-token-download-file-code` folder.

## Decisions

### Root + `pkg/` layout
Yaegi needs `New` and `CreateConfig` on the import path in `.traefik.yml`. Helpers move to:

| Import | Job |
| --- | --- |
| `pkg/dbprovider` | `Provider` interface |
| `pkg/ip2location` | Only implementation: own `DatabaseConfig`, factory, wrapper, auto-update, OpenDB |
| `pkg/dbutils` | BIN header / version / filename date |
| `pkg/fileutils` | exists / copy / search (`TRAEFIK_PLUGIN_GEOBLOCK_PATH`) |
| `pkg/iplookup` | radix CIDR helper + directory monitor |

Alternative: stay one package with files only. Rejected — caller asked for packages. Alternative: underscore names (`file_utils`). Rejected — Go package names.

### Vendor selector and prefixed settings
`databaseProvider` selects the implementation (`ip2location` default; unknown fails). IP2Location-only Traefik keys are `ip2location_*`. The plugin opens the provider through a switch; it does not type-assert a concrete wrapper.

### Provider surface
Narrow interface at plugin scope: initialize (or constructor), `LookupCountry(ip string) (string, error)`, `Close()`. Auto-update is started inside the IP2Location constructor when config says so. Plugin `Lookup` calls `LookupCountry` and keeps the "invalid…" record check if the impl still returns that, or the impl maps it to an error.

`DatabaseWrapper.Get_country_short` stays inside `pkg/ip2location`. Plugin tests that used `*DatabaseWrapper` switch to the interface or the IP2Location type from that package.

### Logger
`createLogger(name, level, format)` → stdout text/json slog. Delete `bufferedFileWriter`. Drop `logBannedRequests` branch in `ServeHTTP`.

### Integration
Remove compose labels for deleted fields. Keep `geoblock-logheaders` with `logStatusDetailHeader` only. Drop remediaiton-header assertions (`X-Geoblock-Action`). Drop `X-Geoblock-Status` asserts.

## Risks / Trade-offs

- [Yaegi import of local `pkg/`] → Mitigation: same pattern as crowdsec; integration compose already mounts the repo as a local plugin.
- [Operators still set removed keys] → Traefik will ignore unknown labels or fail decode depending on version; README states the break.
- [Tests import moved types] → Update imports; keep behavior tests.

## Migration Plan

1. Deploy plugin that lacks the removed fields.
2. Set `logStatusDetailHeader` and keep that name in access logs.
3. Remove file log mounts used only for `logPath`.
4. Rollback: previous plugin version still accepts the old keys.

## Open Questions

None. Explore decisions are in `devstate/explore.md`.
