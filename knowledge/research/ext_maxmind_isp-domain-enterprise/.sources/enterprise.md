---
url: https://dev.maxmind.com/geoip/docs/databases/enterprise/
title: GeoIP Enterprise Databases
fetched: 2026-08-31
authority: official
---

Product: country, region, state, city, ZIP/postal, plus confidence, ISP, domain, and connection type. IP geolocation is imprecise; do not use it to identify a household.

Binary field list is on the binary reference page. Dummy MMDB: `GeoIP2-Enterprise-Test.mmdb`. CSV zip `GeoIP2-Enterprise-CSV_{YYYYMMDD}.zip`.

CSV locations files include `continent_code` (AF, AN, AS, EU, NA, OC, SA), `continent_name`, `country_iso_code`, `country_name`, `subdivision_1_iso_code` / `_name` (least specific; UK example “England” not “Devon”), `subdivision_2_*` (most specific), `city_name`.

Do not use `*_name` fields as map keys. Recommended keys: city → `geoname_id`, continent → `continent_code`, country → `country_iso_code`, subdivisions → `subdivision_{1,2}_iso_code`.

CSV ISP file (joined via `isp_id`) has `isp`, `organization`, `autonomous_system_number`, `autonomous_system_organization`, `connection_type`, `user_type`, MCC/MNC. Blocks CSV has a `domain` column.

May–August 2026 unpacked MMDB size about 339–401 MB.
