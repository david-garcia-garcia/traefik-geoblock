## 1. Presets

- [x] 1.1 Register `maxmind_isp`, `maxmind_domain`, and `maxmind_enterprise` in `pkg/dbwrappers/presets.go` with the maps in design.md. Add `geoip2_isp` / `geoip2_domain` / `geoip2_enterprise` aliases.
- [x] 1.2 Extend `pkg/dbwrappers/fields_test.go` so `PresetNames` and format/type checks cover the new names.

## 2. Operator docs

- [x] 2.1 Add `docs/fields-preconfigured.md` with tables for every shipped preset (IP2Location DB/LITE/ASN, IPinfo Lite/Core/Plus, MaxMind Country/City/ASN/ISP/Domain/Enterprise, aliases) and Record keys.
- [x] 2.2 Replace the `README.md` `### fieldsPreconfigured` package tables with a short definition and a link to `docs/fields-preconfigured.md`. Leave reserved-key and download-URL tables in README.

## 3. Verify

- [x] 3.1 Run `pkg/dbwrappers` tests and confirm README no longer embeds the IP2Location DB1–DB26 table.
