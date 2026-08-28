## Context

See proposal.md Why. The first apply centralized GET only. Three vendor `downloadAndUpdateDatabase` functions still copy lock / temp / hint / dated write. Reserved map keys `geo`/`asn` couple the catalog to a provider role. Traefik already decodes dotted maps (`requestHeaderEnrich`).

## Goals / Non-Goals

**Goals:**

- One download component each provider instantiates.
- Operator-named catalog + explicit provider pointers.
- `databaseType` and `archive` describe the file and the wrap; no provider unpack hook; no content sniff.

**Non-Goals:**

- Changing Lookup / Record mapping.
- Putting Lookup inside the download component.
- New providers.
- Deprecation window for token/code keys.

## Decisions

1. **Named catalog, free keys.** `databaseDownloads` is `map[string]DownloadConfig` with `url`, `headers`, `databaseType`, `archive`. Keys are operator-chosen. Alternative: reserved `geo`/`asn` — rejected after implement (implicit bind, leftover copies).

2. **Explicit pointers.** `ip2location_download_geo`, `ip2location_download_asn`, `ipinfo_download`, `maxmind_download` name a catalog key. Empty pointer = no download. Missing key fails `New`. Unused pointers for another provider are ignored. Alternative: reserved slot names — rejected.

3. **`databaseType` is `bin` or `mmdb`.** After unpack, `bin` dates from the IP2Location BIN header; `mmdb` dates from MMDB `build_epoch`. Not the Lookup provider. Unknown values fail `New`.

4. **`archive` is `none`, `zip`, or `tar.gz`.** How the HTTP body is wrapped. Empty MAY default from a URL path extension (`.zip`, `.tar.gz`/`.tgz`, `.mmdb`, `.bin`). Official IP2Location token and MaxMind permalink URLs have no path extension — those entries MUST set `archive`. No magic-byte sniff. Name locked (not `packFormat`, `suffix`, `unpack`).

5. **One download component.** Owns GET (existing `dbutils.HTTPGet`), lock, temp, unpack-by-`archive`, date-by-`databaseType`, dated write `YYYYMMDD_<catalogKey>.<ext>`, “already current”, ticker, find-latest. Cadence is an argument (30d vs 24h). Provider callback opens/hot-swaps the path. Alternative: provider unpack hook — rejected.

6. **Package.** New `pkg/dbdownload` (loop + archive + type). Reuse `pkg/dbutils` for GET, hint, dated-file find, BIN version, MMDB epoch. Do not copy those helpers.

7. **`databaseAutoUpdateDir` is the one dir.** Required if any bound catalog entry has a URL.

8. **Hard-remove token/code keys.** Same as the first propose. `applyLegacyDatabaseKeys` keeps only `databaseFilePath` → `ip2location_databaseFilePath`.

9. **Seed paths stay prefixed.** Bundled seeds unchanged. Core/Plus/City are operator URLs or paths, not codes.

## Risks / Trade-offs

- [Operators lose free IP2Location LITE with empty token] → Document the official lite ZIP URL; set `archive: zip`, `databaseType: bin`.
- [Old dated filenames are ignored] → Document re-download; globs use catalog keys.
- [MaxMind Basic Auth is on the operator] → README shows `Authorization: Basic`. Plugin does not parse `accountId:licenseKey`.
- [Empty `archive` + extension-less URL] → Fail with `DownloadHint`; operator sets `archive`.
- [Yaegi + nested map of structs] → Same decode path as `requestHeaderEnrich`.

## Migration Plan

1. Rewrite compose and README to catalog entries (`url`, `databaseType`, `archive`, optional `headers`) plus pointers and `databaseAutoUpdateDir`.
2. Remove token/code env vars from the plugin contract (compose may still interpolate a token into a URL).
3. Existing auto-update directories: start empty or accept that old names are not chosen.
4. Rollback: revert the change; old keys return. No dual-read of token/code.

## Open Questions

None that change this design. Explore assumed rows were taken as the decisions above when the human said continue.
