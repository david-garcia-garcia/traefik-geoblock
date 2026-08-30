## Context

Lookup today is one `databaseProvider` plus vendor pointers into `databaseSources`. Bundled `seeds/` names live on vendor `DefaultFileName`. IP2Location geo+ASN is the only two-file merge. Go map iteration is not a stable merge order.

## Goals / Non-Goals

**Goals:**

- Catalog rows are the operator model (`vendor`, `defaultFile`, `enabled`).
- Shipped `seeds/` and reserved download URLs are catalog rows inserted in `Prepare` when the key is absent.
- Merge N enabled sources; first non-empty meta key wins (lexicographic key order).
- Wrappers and reclaim stay.

**Non-Goals:**

- YAML column lists.
- Runtime schema sniff of BIN/MMDB.
- Renaming `core_geoblock_database_provider` (note on `devstate/issues.md`).
- Changing `mode` / `countryHeader`.

## Decisions

- `vendor` on the row: `ip2location` | `ip2location-asn` | `ipinfo` | `maxmind`. Not `databaseType`.
- `enabled` is `*bool`. Nil / omitted = enabled. Shipped extras insert `enabled: false`.
- `defaultFile` is the basename Resolve already searches under `seeds/`.
- One internal `dbprovider.Provider` walks enabled sources. Plugin `Lookup` unchanged.
- IP2Location ASN is its own `vendor` (`LookupASN`), not a second field on the geo constructor.
- Skip a source that errors; if every enabled source errors, return error (`banIfError`).
- Empty `vendor` or format mismatch on an enabled row fails `Prepare`.
- Zero enabled rows in a lookup mode fails `Prepare`.

## Risks / Trade-offs

- **BREAKING** YAML that set `databaseProvider` or pointers.
- Lexicographic merge is deterministic but surprising if keys are poorly named. No priority field yet.
- `*bool` must decode from Traefik YAML (`enabled: false`).
- Enabling several country sources overwrites nothing after the first key; operators must disable the shipped IP2Location default when they want IPinfo-only.

## Migration Plan

Operators who used `databaseProvider: ipinfo` and `ipinfo_source: lite` set `databaseSources.lite.vendor: ipinfo`, `enabled: true`, and `databaseSources.default_ip2location.enabled: false`. Same pattern for MaxMind. ASN: add a row with `vendor: ip2location-asn`.
