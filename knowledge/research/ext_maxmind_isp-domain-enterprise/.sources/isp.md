---
url: https://dev.maxmind.com/geoip/docs/databases/isp/
title: GeoIP ISP Databases
fetched: 2026-08-31
authority: official
---

Product: determine ISP, organization name, and ASN / ASN organization for an IP.

Binary uses MaxMind DB. Field list is on the binary reference page. Dummy MMDB: `GeoIP2-ISP-Test.mmdb` on maxmind/MaxMind-DB. CSV zip `GeoIP2-ISP-CSV_{YYYYMMDD}.zip` with `GeoIP2-ISP-Blocks-IPv4.csv` / `IPv6.csv`. CSV columns: `network`, `isp`, `organization`, `autonomous_system_number`, `autonomous_system_organization`, `mobile_country_code`, `mobile_network_code`.

May–August 2026 unpacked MMDB size about 19.8–20.3 MB. New fields may be added at any time (CSV: new columns to the right).
