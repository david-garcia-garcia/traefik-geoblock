## Why

The operator-facing `fieldsPreconfigured` tables live in `README.md` for IP2Location only. IPinfo and MaxMind presets are a name list with no column map. Official GeoIP2 ISP, Domain, and Enterprise files have Record-mappable paths that are not registered names.

## What Changes

- Move the `fieldsPreconfigured` package tables out of `README.md` into `docs/fields-preconfigured.md`. README keeps a short definition and links to that file.
- Document every shipped preset (IP2Location, IPinfo Lite/Core/Plus, MaxMind Country/City/ASN plus the new names below) with Record keys, format, and download hint.
- Register `maxmind_isp`, `maxmind_domain`, and `maxmind_enterprise` (and `geoip2_*` aliases) as `fieldsPreconfigured` values. Maps follow official GeoIP2 binary paths onto existing Record keys.
- `ipinfo_plus` stays a Core clone. No new Record keys. No new vendor.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `core_geoblock_database_source-catalog`: Operator-facing preset tables live in `docs/fields-preconfigured.md`; README links there and does not embed those tables.
- `core_geoblock_database_maxmind-geolite`: Named presets include `maxmind_isp`, `maxmind_domain`, and `maxmind_enterprise` (plus `geoip2_*` aliases).

## Impact

- `README.md` — replace `### fieldsPreconfigured` tables with a pointer.
- `docs/fields-preconfigured.md` — new operator file.
- `pkg/dbwrappers/presets.go` — ISP / Domain / Enterprise maps and aliases.
- `pkg/dbwrappers/fields_test.go` — new names in `PresetNames`.
- `openspec/specs/core_geoblock_database_source-catalog` and `core_geoblock_database_maxmind-geolite`.
- Usage packet `knowledge/devdocs/core_geoblock_database_source.md` if the preset list is incomplete.
