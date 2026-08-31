---
url: https://dev.maxmind.com/geoip/docs/databases/enterprise/binary/
title: GeoIP Enterprise binary database fields
fetched: 2026-08-31
authority: official
---

Top-level record is a map. Empty/undefined keys are omitted. Top-level keys: `city`, `continent`, `country`, `location`, `postal`, `registered_country`, `represented_country`, `subdivisions` (array of maps, largest to smallest), `traits`.

`city`: `confidence` (uint16 0–100), `geoname_id` (uint32), `names` (locale → localized name).

`continent`: `code` (two-character, e.g. NA, OC), `geoname_id`, `names`.

`country` (located country): `confidence`, `geoname_id`, `is_in_european_union` (present only when true), `iso_code` (ISO 3166-1 alpha), `names`.

`subdivisions`: `confidence`, `geoname_id`, `iso_code` (up to three characters, ISO 3166-2 subdivision portion), `names`.

`traits`: `autonomous_system_number` (uint32), `autonomous_system_organization` (string), `connection_type` (Cable/DSL, Cellular, Corporate, Satellite; more may be added), `domain` (second-level, same definition as Domain product), `is_anycast` (present only when true), `isp` (ISP name), `mobile_country_code`, `mobile_network_code`, `organization`, `user_type` (closed list including business, residential, hosting, …).

`registered_country` also has `iso_code` / `names` (country where the ISP registered the block; may differ from located `country`).
