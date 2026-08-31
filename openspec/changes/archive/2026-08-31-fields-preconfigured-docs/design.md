## Context

See proposal.md for why. `pkg/dbwrappers/presets.go` already registers IP2Location DB/LITE/ASN, IPinfo Lite/Core/Plus, and MaxMind Country/City/ASN. MMDB decode already walks dotted paths (`country.iso_code`, `subdivisions.0.iso_code`). Record keys stay the nine in `pkg/dbprovider/provider.go`. Official GeoIP2 ISP / Domain / Enterprise binary field pages own the new paths.

## Goals / Non-Goals

**Goals:**

- One operator file `docs/fields-preconfigured.md` is the table host.
- README `### fieldsPreconfigured` is a definition plus link.
- New MaxMind names use the same `register` / `registerAlias` path as Country/City/ASN.

**Non-Goals:**

- New Record keys (lat/long, postal, privacy flags).
- IPinfo Standard / Privacy / Company presets.
- Changing reserved catalog rows or download URLs.
- Bundled ISP/Domain/Enterprise seed files (paid; no official dummy in this repo).

## Decisions

- **Filename `docs/fields-preconfigured.md`.** Alternative: `docs/fieldsPreconfigured.md` (YAML key). Rejected: repo markdown leaves are kebab-case (`README.md` excepted).
- **ISP map: `isp` + ASN number only.** Official ISP also has `organization` and `autonomous_system_organization`. Mapping two strings onto Record `isp` would last-write-win on Go map iteration. Keep `isp` as the ISP name; ASN org stays unused (same as unused BIN extras). Alternative: map org → isp when isp empty — rejected (needs apply-order rules).
- **Enterprise traits, not top-level isp.** Official Enterprise binary puts `isp` / `domain` / `autonomous_system_number` under `traits`. City nest stays at the Country/City paths. Alternative: reuse `maxmind_city` plus `maxmind_isp` on two catalog rows — rejected (one file, one row).
- **Plus stays Core clone.** Official Plus Record-mappable columns match Core. Extra Plus columns have no Record key.
- **Aliases only `geoip2_*`.** No `geolite2_isp` (GeoLite has no ISP/Domain/Enterprise editions).

## Risks / Trade-offs

- [Docs drift] → Mitigation: tables are written from `PresetNames()` plus the maps in `presets.go` in the same change; tests assert the new names exist.
- [Operator opens ISP file with `maxmind_country`] → Mitigation: unused paths stay empty; docs say which preset matches which edition.
- [No fixture for ISP/Domain/Enterprise] → Mitigation: unit-test registration and Field types only; do not add a live GeoIP2 file.

## Migration Plan

No YAML break. Existing preset names keep the same maps. Operators who already wrote a custom `fields` map for GeoIP2 ISP can switch to `fieldsPreconfigured: maxmind_isp`. Rollback is revert the docs and the three `register` calls.
