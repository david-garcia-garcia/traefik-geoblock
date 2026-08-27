---
url: https://dev.maxmind.com/geoip/docs/databases/city-and-country/
title: GeoIP and GeoLite City and Country Databases
fetched: 2026-08-27
authority: official
---

Binary uses MaxMind DB. Official client APIs are listed on the databases page; unofficial APIs are unsupported.

CSV zip name: `{GeoIP2,GeoLite2}-{City,Country}-CSV_{YYYYMMDD}.zip`. Locations CSV column `country_iso_code` is string(2), ISO 3166-1, included in Country and City. Do not use `*_name` fields as map keys; recommended country key is `country_iso_code`.

CSV `is_anycast` is empty in GeoLite2-Country and GeoLite2-City.

Example MMDB fixtures (dummy data, not real GeoIP): `GeoIP2-City-Test.mmdb` and `GeoIP2-Country-Test.mmdb` in maxmind/MaxMind-DB `test-data/`.

May–August 2026 unpacked GeoLite Country MMDB size range: 8.61–9.69 MB. GeoLite City MMDB: 43.9–66.9 MB.
