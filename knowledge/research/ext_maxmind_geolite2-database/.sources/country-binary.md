---
url: https://dev.maxmind.com/geoip/docs/databases/city-and-country/country-binary/
title: GeoIP Country binary database fields
fetched: 2026-08-27
authority: official
---

Top-level record is a map. Keys that map to empty/undefined values are omitted.

Top-level keys: `continent` (map), `country` (map — country where MaxMind believes the IP is located), `registered_country` (map — country where the ISP registered the block), `represented_country` (map), `traits` (map).

`country` map fields:

- `geoname_id` (uint32)
- `is_in_european_union` (boolean; present only when true)
- `iso_code` (string) — two-character ISO 3166-1 alpha code
- `names` (map of locale → localized name)

`registered_country` and `represented_country` also have `iso_code`. `traits` includes `is_anycast` (present only when true).
