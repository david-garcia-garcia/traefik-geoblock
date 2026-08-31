## ADDED Requirements

### Requirement: Operator preset tables live in docs
The operator-facing tables of `fieldsPreconfigured` names, vendor packages, and Record keys SHALL live in `docs/fields-preconfigured.md`. That file SHALL list every shipped preset name (including aliases). `README.md` SHALL link to `docs/fields-preconfigured.md` from the `fieldsPreconfigured` section and MUST NOT embed the IP2Location package tables.

#### Scenario: README points at the docs file
- **WHEN** an operator opens `README.md` at the `fieldsPreconfigured` heading
- **THEN** that section links to `docs/fields-preconfigured.md`
- **AND** that section does not contain the IP2Location DB1–DB26 package table

#### Scenario: Docs file lists IPinfo and MaxMind presets
- **WHEN** an operator opens `docs/fields-preconfigured.md`
- **THEN** the file includes tables for `ipinfo_lite`, `ipinfo_core`, `ipinfo_plus`, `maxmind_country`, `maxmind_city`, `maxmind_asn`, `maxmind_isp`, `maxmind_domain`, and `maxmind_enterprise`
- **AND** each row names the Record keys that preset fills
